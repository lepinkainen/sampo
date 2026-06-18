package ocr

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store manages OCR results in SQLite.
type Store struct {
	db *sql.DB
}

type storedRow struct {
	RootID      string
	RelPath     string
	Canonical   string
	Mtime       int64
	Size        int64
	ModelVer    string
	ScannedAt   time.Time
	Text        string
	BlocksJSON  string
	NeedsChange bool
}

// NewStore opens or creates the OCR database.
func NewStore(cacheDir string) (*Store, error) {
	dbPath := filepath.Join(cacheDir, "ocr.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening ocr db: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ocr (
			root_id    TEXT NOT NULL,
			rel_path   TEXT NOT NULL,
			mtime      INTEGER NOT NULL,
			size       INTEGER NOT NULL,
			model_ver  TEXT NOT NULL,
			scanned_at DATETIME NOT NULL,
			text       TEXT NOT NULL,
			blocks     TEXT NOT NULL,
			PRIMARY KEY (root_id, rel_path)
		);
		CREATE INDEX IF NOT EXISTS idx_ocr_dir ON ocr(root_id, rel_path);
	`)
	if err != nil {
		return fmt.Errorf("migrating ocr db: %w", err)
	}
	if err := normalizeExistingRelPaths(db); err != nil {
		return fmt.Errorf("normalizing ocr paths: %w", err)
	}
	return nil
}

func normalizeExistingRelPaths(db *sql.DB) error {
	rows, err := db.Query(
		`SELECT root_id, rel_path, mtime, size, model_ver, scanned_at, text, blocks FROM ocr`,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	byKey := make(map[string]storedRow)
	changed := false
	for rows.Next() {
		var row storedRow
		scanErr := rows.Scan(&row.RootID, &row.RelPath, &row.Mtime, &row.Size, &row.ModelVer, &row.ScannedAt, &row.Text, &row.BlocksJSON)
		if scanErr != nil {
			return scanErr
		}
		row.Canonical = NormalizeRelPath(row.RelPath)
		row.NeedsChange = row.RelPath != row.Canonical
		changed = changed || row.NeedsChange

		key := row.RootID + "\x00" + row.Canonical
		if existing, ok := byKey[key]; !ok || betterStoredRow(row, existing) {
			if ok {
				changed = true
			}
			byKey[key] = row
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		_ = rows.Close()
		return rowsErr
	}
	if closeErr := rows.Close(); closeErr != nil {
		return closeErr
	}
	if !changed {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, execErr := tx.Exec(`DELETE FROM ocr`); execErr != nil {
		return execErr
	}
	stmt, err := tx.Prepare(
		`INSERT INTO ocr (root_id, rel_path, mtime, size, model_ver, scanned_at, text, blocks)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for _, row := range byKey {
		if _, err := stmt.Exec(row.RootID, row.Canonical, row.Mtime, row.Size, row.ModelVer, row.ScannedAt, row.Text, row.BlocksJSON); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func betterStoredRow(candidate, existing storedRow) bool {
	if candidate.ScannedAt.After(existing.ScannedAt) {
		return true
	}
	if existing.ScannedAt.After(candidate.ScannedAt) {
		return false
	}
	if !candidate.NeedsChange && existing.NeedsChange {
		return true
	}
	return false
}

// Put inserts or replaces an OCR result.
func (s *Store) Put(result *Result) error {
	blocksJSON, err := json.Marshal(result.Blocks)
	if err != nil {
		return fmt.Errorf("marshaling blocks: %w", err)
	}

	relPath := NormalizeRelPath(result.RelPath)
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO ocr (root_id, rel_path, mtime, size, model_ver, scanned_at, text, blocks)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		result.RootID, relPath, result.Mtime, result.Size, result.ModelVer,
		result.ScannedAt, result.Text, string(blocksJSON),
	)
	if err != nil {
		return fmt.Errorf("upserting ocr result: %w", err)
	}
	return nil
}

// Get retrieves an OCR result for a single file, or nil if not present.
func (s *Store) Get(rootID, relPath string) (*Result, error) {
	relPath = NormalizeRelPath(relPath)
	row := s.db.QueryRow(
		`SELECT root_id, rel_path, mtime, size, model_ver, scanned_at, text, blocks
		 FROM ocr WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	)

	var r Result
	var blocksJSON string
	err := row.Scan(&r.RootID, &r.RelPath, &r.Mtime, &r.Size, &r.ModelVer, &r.ScannedAt, &r.Text, &blocksJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying ocr: %w", err)
	}
	r.RelPath = NormalizeRelPath(r.RelPath)
	if err := json.Unmarshal([]byte(blocksJSON), &r.Blocks); err != nil {
		return nil, fmt.Errorf("unmarshaling blocks: %w", err)
	}
	return &r, nil
}

// IsStale checks if a file changed (mtime/size) or was OCR'd with a different
// model version since its last scan.
func (s *Store) IsStale(rootID, relPath string, mtime, size int64, modelVer string) bool {
	var storedMtime, storedSize int64
	var storedModelVer string
	relPath = NormalizeRelPath(relPath)
	err := s.db.QueryRow(
		`SELECT mtime, size, model_ver FROM ocr WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	).Scan(&storedMtime, &storedSize, &storedModelVer)
	if err != nil {
		return true
	}
	return storedMtime != mtime || storedSize != size || storedModelVer != modelVer
}

// dirPrefix normalizes a directory path for prefix matching.
func dirPrefix(dirPath string) string {
	dirPath = NormalizeRelPath(dirPath)
	if dirPath == "" {
		return ""
	}
	return dirPath + "/"
}

// SearchByText returns rel paths whose recognized text contains the query
// substring (case-insensitive), scoped to files under dirPath.
func (s *Store) SearchByText(rootID, dirPath, query string) ([]string, error) {
	prefix := dirPrefix(dirPath)
	pattern := "%" + strings.ToLower(query) + "%"

	rows, err := s.db.Query(
		`SELECT rel_path FROM ocr
		 WHERE root_id = ? AND rel_path LIKE ? AND LOWER(text) LIKE ?`,
		rootID, prefix+"%", pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("searching ocr text: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var paths []string
	for rows.Next() {
		var relPath string
		if err := rows.Scan(&relPath); err != nil {
			return nil, err
		}
		paths = append(paths, NormalizeRelPath(relPath))
	}
	return paths, rows.Err()
}

// isDirectChild returns true if relPath is a direct child under prefix (not nested).
func isDirectChild(relPath, prefix string) bool {
	relPath = NormalizeRelPath(relPath)
	if prefix != "" {
		if !strings.HasPrefix(relPath, prefix) {
			return false
		}
		relPath = strings.TrimPrefix(relPath, prefix)
	}
	return relPath != "" && !strings.Contains(relPath, "/")
}

// GetDirText returns a map of relPath -> recognized text for scanned direct
// children of a directory (empty text entries are omitted).
func (s *Store) GetDirText(rootID, dirPath string) (map[string]string, error) {
	prefix := dirPrefix(dirPath)

	rows, err := s.db.Query(
		`SELECT rel_path, text FROM ocr
		 WHERE root_id = ? AND rel_path LIKE ? AND text != ''`,
		rootID, prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("getting dir ocr text: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]string)
	for rows.Next() {
		var relPath, text string
		if err := rows.Scan(&relPath, &text); err != nil {
			return nil, err
		}
		if isDirectChild(relPath, prefix) {
			result[NormalizeRelPath(relPath)] = text
		}
	}
	return result, rows.Err()
}

// GetText returns the recognized text for a single file (empty if none).
func (s *Store) GetText(rootID, relPath string) (string, error) {
	var text string
	relPath = NormalizeRelPath(relPath)
	err := s.db.QueryRow(
		`SELECT text FROM ocr WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	).Scan(&text)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting ocr text: %w", err)
	}
	return text, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}
