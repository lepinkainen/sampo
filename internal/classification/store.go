package classification

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store manages classification results in SQLite.
type Store struct {
	db *sql.DB
}

// NewStore opens or creates the classification database.
func NewStore(cacheDir string) (*Store, error) {
	dbPath := filepath.Join(cacheDir, "classification.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("opening classification db: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS classifications (
			root_id    TEXT NOT NULL,
			rel_path   TEXT NOT NULL,
			mtime      INTEGER NOT NULL,
			size       INTEGER NOT NULL,
			model_ver  TEXT NOT NULL,
			scanned_at DATETIME NOT NULL,
			sha256     TEXT,
			crc32      TEXT,
			PRIMARY KEY (root_id, rel_path)
		);
		CREATE TABLE IF NOT EXISTS tags (
			root_id  TEXT NOT NULL,
			rel_path TEXT NOT NULL,
			label    TEXT NOT NULL,
			score    REAL NOT NULL,
			PRIMARY KEY (root_id, rel_path, label),
			FOREIGN KEY (root_id, rel_path) REFERENCES classifications(root_id, rel_path) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_tags_label ON tags(root_id, label);
		CREATE INDEX IF NOT EXISTS idx_tags_dir ON tags(root_id, rel_path);
	`)
	if err != nil {
		return fmt.Errorf("migrating classification db: %w", err)
	}

	// Migration: add sha256 and crc32 columns if they don't exist
	for _, col := range []string{"sha256", "crc32"} {
		_, _ = db.Exec("ALTER TABLE classifications ADD COLUMN " + col + " TEXT")
	}

	return nil
}

// Put inserts or replaces a classification result and its tags in a transaction.
func (s *Store) Put(result *Result) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing tags first (cascade would handle this on DELETE, but we're doing REPLACE)
	_, err = tx.Exec(
		`DELETE FROM tags WHERE root_id = ? AND rel_path = ?`,
		result.RootID, result.RelPath,
	)
	if err != nil {
		return fmt.Errorf("deleting old tags: %w", err)
	}

	_, err = tx.Exec(
		`INSERT OR REPLACE INTO classifications (root_id, rel_path, mtime, size, model_ver, scanned_at, sha256, crc32)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		result.RootID, result.RelPath, result.Mtime, result.Size, result.ModelVer, result.ScannedAt,
		nullString(result.SHA256), nullString(result.CRC32),
	)
	if err != nil {
		return fmt.Errorf("upserting classification: %w", err)
	}

	for _, tag := range result.Tags {
		_, err = tx.Exec(
			`INSERT INTO tags (root_id, rel_path, label, score) VALUES (?, ?, ?, ?)`,
			result.RootID, result.RelPath, tag.Label, tag.Score,
		)
		if err != nil {
			return fmt.Errorf("inserting tag %s: %w", tag.Label, err)
		}
	}

	return tx.Commit()
}

// nullString converts an empty string to nil for nullable TEXT columns.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Get retrieves a classification result with tags for a single file.
func (s *Store) Get(rootID, relPath string) (*Result, error) {
	row := s.db.QueryRow(
		`SELECT root_id, rel_path, mtime, size, model_ver, scanned_at, sha256, crc32
		 FROM classifications WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	)

	var r Result
	var sha256Val, crc32Val sql.NullString
	err := row.Scan(&r.RootID, &r.RelPath, &r.Mtime, &r.Size, &r.ModelVer, &r.ScannedAt, &sha256Val, &crc32Val)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying classification: %w", err)
	}
	r.SHA256 = sha256Val.String
	r.CRC32 = crc32Val.String

	tags, err := s.getTags(rootID, relPath)
	if err != nil {
		return nil, err
	}
	r.Tags = tags

	return &r, nil
}

func (s *Store) getTags(rootID, relPath string) ([]TagScore, error) {
	rows, err := s.db.Query(
		`SELECT label, score FROM tags WHERE root_id = ? AND rel_path = ? ORDER BY score DESC`,
		rootID, relPath,
	)
	if err != nil {
		return nil, fmt.Errorf("querying tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tags []TagScore
	for rows.Next() {
		var t TagScore
		if err := rows.Scan(&t.Label, &t.Score); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// IsStale checks if a file has changed (mtime/size) or was classified with a
// different model version since its last classification.
func (s *Store) IsStale(rootID, relPath string, mtime int64, size int64, modelVer string) bool {
	var storedMtime, storedSize int64
	var storedModelVer string
	err := s.db.QueryRow(
		`SELECT mtime, size, model_ver FROM classifications WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	).Scan(&storedMtime, &storedSize, &storedModelVer)
	if err != nil {
		return true
	}
	return storedMtime != mtime || storedSize != size || storedModelVer != modelVer
}

// dirPrefix normalizes a directory path for prefix matching.
func dirPrefix(dirPath string) string {
	if dirPath != "" && !strings.HasSuffix(dirPath, "/") {
		return dirPath + "/"
	}
	return dirPath
}

// isDirectChild returns true if relPath is a direct child under prefix (not nested).
func isDirectChild(relPath, prefix string) bool {
	rel := strings.TrimPrefix(relPath, prefix)
	return !strings.Contains(rel, "/")
}

// GetDirTags returns a map of relPath -> []TagScore for all scanned direct children of a directory.
func (s *Store) GetDirTags(rootID, dirPath string) (map[string][]TagScore, error) {
	prefix := dirPrefix(dirPath)

	rows, err := s.db.Query(
		`SELECT t.rel_path, t.label, t.score
		 FROM tags t
		 INNER JOIN classifications c ON t.root_id = c.root_id AND t.rel_path = c.rel_path
		 WHERE t.root_id = ? AND t.rel_path LIKE ?
		 ORDER BY t.rel_path, t.score DESC`,
		rootID, prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("getting dir tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]TagScore)
	for rows.Next() {
		var relPath, label string
		var score float32
		if err := rows.Scan(&relPath, &label, &score); err != nil {
			return nil, err
		}
		if isDirectChild(relPath, prefix) {
			result[relPath] = append(result[relPath], TagScore{Label: label, Score: score})
		}
	}
	return result, rows.Err()
}

// FilterByTag returns a set of relPaths that have the given tag with score >= minScore,
// limited to direct children of dirPath.
func (s *Store) FilterByTag(rootID, dirPath, label string, minScore float32) (map[string]bool, error) {
	prefix := dirPrefix(dirPath)

	rows, err := s.db.Query(
		`SELECT t.rel_path FROM tags t
		 WHERE t.root_id = ? AND t.rel_path LIKE ? AND t.label = ? AND t.score >= ?`,
		rootID, prefix+"%", label, minScore,
	)
	if err != nil {
		return nil, fmt.Errorf("filtering by tag: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]bool)
	for rows.Next() {
		var relPath string
		if err := rows.Scan(&relPath); err != nil {
			return nil, err
		}
		if isDirectChild(relPath, prefix) {
			result[relPath] = true
		}
	}
	return result, rows.Err()
}

// SearchByTag returns rel paths where any tag label contains the query substring,
// scoped to files under dirPath.
func (s *Store) SearchByTag(rootID, dirPath, query string) ([]string, error) {
	prefix := dirPrefix(dirPath)
	pattern := "%" + query + "%"

	rows, err := s.db.Query(
		`SELECT DISTINCT t.rel_path FROM tags t
		 WHERE t.root_id = ? AND t.rel_path LIKE ? AND LOWER(t.label) LIKE ?`,
		rootID, prefix+"%", pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("searching by tag: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var paths []string
	for rows.Next() {
		var relPath string
		if err := rows.Scan(&relPath); err != nil {
			return nil, err
		}
		paths = append(paths, relPath)
	}
	return paths, rows.Err()
}

// GetFileTags returns tags for a single file (by exact rel_path).
func (s *Store) GetFileTags(rootID, relPath string) ([]TagScore, error) {
	return s.getTags(rootID, relPath)
}

// ScannedAt returns the scan time for a file, or zero if not scanned.
func (s *Store) ScannedAt(rootID, relPath string) time.Time {
	var t time.Time
	_ = s.db.QueryRow(
		`SELECT scanned_at FROM classifications WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	).Scan(&t)
	return t
}

// ChecksumInfo holds hash information for a file.
type ChecksumInfo struct {
	SHA256 string `json:"sha256,omitempty"`
	CRC32  string `json:"crc32,omitempty"`
}

// GetDirChecksums returns a map of relPath -> ChecksumInfo for scanned files under dirPath.
func (s *Store) GetDirChecksums(rootID, dirPath string) (map[string]ChecksumInfo, error) {
	prefix := dirPrefix(dirPath)

	rows, err := s.db.Query(
		`SELECT rel_path, sha256, crc32 FROM classifications
		 WHERE root_id = ? AND rel_path LIKE ? AND (sha256 IS NOT NULL OR crc32 IS NOT NULL)`,
		rootID, prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("getting dir checksums: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]ChecksumInfo)
	for rows.Next() {
		var relPath string
		var sha256Val, crc32Val sql.NullString
		if err := rows.Scan(&relPath, &sha256Val, &crc32Val); err != nil {
			return nil, err
		}
		if isDirectChild(relPath, prefix) {
			result[relPath] = ChecksumInfo{SHA256: sha256Val.String, CRC32: crc32Val.String}
		}
	}
	return result, rows.Err()
}

// DuplicateGroup holds a group of files with matching hashes.
type DuplicateGroup struct {
	Hash     string    `json:"hash"`
	HashType string    `json:"hashType"`
	Size     int64     `json:"size"`
	Files    []DupFile `json:"files"`
}

// DupFile identifies a file in a duplicate group.
type DupFile struct {
	RootID string `json:"rootId"`
	Path   string `json:"path"`
}

// FindDuplicates finds files with matching SHA256 under a directory.
func (s *Store) FindDuplicates(rootID, dirPath string) ([]DuplicateGroup, error) {
	prefix := dirPrefix(dirPath)

	rows, err := s.db.Query(
		`SELECT c1.sha256, c1.size, c1.rel_path
		 FROM classifications c1
		 WHERE c1.root_id = ? AND c1.rel_path LIKE ? AND c1.sha256 IS NOT NULL
		   AND c1.sha256 IN (
		     SELECT sha256 FROM classifications
		     WHERE root_id = ? AND rel_path LIKE ? AND sha256 IS NOT NULL
		     GROUP BY sha256 HAVING COUNT(*) > 1
		   )
		 ORDER BY c1.sha256`,
		rootID, prefix+"%", rootID, prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("finding duplicates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groups := make(map[string]*DuplicateGroup)
	var order []string
	for rows.Next() {
		var hash, relPath string
		var size int64
		if err := rows.Scan(&hash, &size, &relPath); err != nil {
			return nil, err
		}
		g, ok := groups[hash]
		if !ok {
			g = &DuplicateGroup{Hash: hash, HashType: "sha256", Size: size}
			groups[hash] = g
			order = append(order, hash)
		}
		g.Files = append(g.Files, DupFile{RootID: rootID, Path: relPath})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]DuplicateGroup, 0, len(order))
	for _, hash := range order {
		result = append(result, *groups[hash])
	}
	return result, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}
