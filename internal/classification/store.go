package classification

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/lepinkainen/sampo/internal/filesystem"
)

// Store manages classification results in SQLite.
type Store struct {
	db *sql.DB
}

// NewStore opens or creates the classification database.
func NewStore(cacheDir string) (*Store, error) {
	dbPath := filepath.Join(cacheDir, "classification.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening classification db: %w", err)
	}
	// SQLite allows only one writer; a single connection serializes writes
	// in-process instead of failing with SQLITE_BUSY under concurrent Puts.
	db.SetMaxOpenConns(1)

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

	// Migration: add columns if they don't exist
	for _, col := range []string{"sha256 TEXT", "crc32 TEXT", "phash TEXT", "width INTEGER", "height INTEGER"} {
		_, _ = db.Exec("ALTER TABLE classifications ADD COLUMN " + col)
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
		`INSERT INTO classifications (root_id, rel_path, mtime, size, model_ver, scanned_at, sha256, crc32, phash, width, height)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(root_id, rel_path) DO UPDATE SET
		     mtime = excluded.mtime,
		     size = excluded.size,
		     model_ver = excluded.model_ver,
		     scanned_at = excluded.scanned_at,
		     sha256 = excluded.sha256,
		     crc32 = COALESCE(excluded.crc32, classifications.crc32),
		     phash = excluded.phash,
		     width = excluded.width,
		     height = excluded.height`,
		result.RootID, result.RelPath, result.Mtime, result.Size, result.ModelVer, result.ScannedAt,
		nullString(result.SHA256), nullString(result.CRC32),
		nullString(result.PHash), nullInt(result.Width), nullInt(result.Height),
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

// nullInt converts a zero value to nil for nullable INTEGER columns.
func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// modelVerFilename is the sentinel model_ver for rows that carry only a
// filename-derived CRC32 (videotagger-style filenames). It marks the row as
// not-yet-classified so IsStale returns true when the real classifier runs.
const modelVerFilename = "filename"

// FilenameCRC32Entry pairs a file's identity with a CRC32 parsed from its name.
type FilenameCRC32Entry struct {
	RelPath string
	Mtime   int64
	Size    int64
	CRC32   string
}

// PutFilenameCRC32Batch upserts filename-derived CRC32 values for multiple
// files in a single transaction. Each row is seeded with model_ver="filename"
// so it participates in duplicate detection before classification runs. When a
// row already exists (from a prior classification), only crc32 is updated —
// tags, sha256, phash, and dimensions are left untouched. A nil/empty CRC32
// on the excluded value preserves the existing crc32.
func (s *Store) PutFilenameCRC32Batch(rootID string, entries []FilenameCRC32Entry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO classifications (root_id, rel_path, mtime, size, model_ver, scanned_at, crc32)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(root_id, rel_path) DO UPDATE SET
		     crc32 = COALESCE(excluded.crc32, classifications.crc32)`,
	)
	if err != nil {
		return fmt.Errorf("preparing filename crc32 upsert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().UTC()
	for _, e := range entries {
		if _, err := stmt.Exec(rootID, e.RelPath, e.Mtime, e.Size, modelVerFilename, now, nullString(e.CRC32)); err != nil {
			return fmt.Errorf("upserting filename crc32 for %s: %w", e.RelPath, err)
		}
	}
	return tx.Commit()
}

// Get retrieves a classification result with tags for a single file.
func (s *Store) Get(rootID, relPath string) (*Result, error) {
	row := s.db.QueryRow(
		`SELECT root_id, rel_path, mtime, size, model_ver, scanned_at, sha256, crc32, phash, width, height
		 FROM classifications WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	)

	var r Result
	var sha256Val, crc32Val, phashVal sql.NullString
	var widthVal, heightVal sql.NullInt64
	err := row.Scan(&r.RootID, &r.RelPath, &r.Mtime, &r.Size, &r.ModelVer, &r.ScannedAt, &sha256Val, &crc32Val, &phashVal, &widthVal, &heightVal)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying classification: %w", err)
	}
	r.SHA256 = sha256Val.String
	r.CRC32 = crc32Val.String
	r.PHash = phashVal.String
	r.Width = int(widthVal.Int64)
	r.Height = int(heightVal.Int64)

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

// escapeLike escapes LIKE wildcards so path characters match literally.
// Queries using it must append `ESCAPE '\'`.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// pathVariants returns both cache-key spellings used by older and newer callers.
// Some rows were written with a leading slash from FileEntry.Path, while tests
// and some direct scanners use root-relative paths without one.
func pathVariants(relPath string) []string {
	trimmed := strings.Trim(relPath, "/")
	if trimmed == "" {
		return nil
	}
	return []string{trimmed, "/" + trimmed}
}

type scopedPathMatch struct {
	first        string
	firstPrefix  string
	second       string
	secondPrefix string
	ok           bool
}

func newScopedPathMatch(relPath string) scopedPathMatch {
	variants := pathVariants(relPath)
	if len(variants) == 0 {
		return scopedPathMatch{}
	}
	return scopedPathMatch{
		first:        variants[0],
		firstPrefix:  escapeLike(filesystem.DirPrefix(variants[0])) + "%",
		second:       variants[1],
		secondPrefix: escapeLike(filesystem.DirPrefix(variants[1])) + "%",
		ok:           true,
	}
}

// GetDirTags returns a map of relPath -> []TagScore for all scanned direct children of a directory.
func (s *Store) GetDirTags(rootID, dirPath string) (map[string][]TagScore, error) {
	prefix := filesystem.DirPrefix(dirPath)

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
		if filesystem.IsDirectChild(relPath, prefix) {
			result[relPath] = append(result[relPath], TagScore{Label: label, Score: score})
		}
	}
	return result, rows.Err()
}

// FilterByTag returns a set of relPaths that have the given tag with score >= minScore,
// limited to direct children of dirPath.
func (s *Store) FilterByTag(rootID, dirPath, label string, minScore float32) (map[string]bool, error) {
	prefix := filesystem.DirPrefix(dirPath)

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
		if filesystem.IsDirectChild(relPath, prefix) {
			result[relPath] = true
		}
	}
	return result, rows.Err()
}

// SearchByTag returns rel paths where any tag label contains the query substring,
// scoped to files under dirPath.
func (s *Store) SearchByTag(rootID, dirPath, query string) ([]string, error) {
	prefix := filesystem.DirPrefix(dirPath)
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
	prefix := filesystem.DirPrefix(dirPath)

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
		if filesystem.IsDirectChild(relPath, prefix) {
			result[relPath] = ChecksumInfo{SHA256: sha256Val.String, CRC32: crc32Val.String}
		}
	}
	return result, rows.Err()
}

// DuplicateGroup holds a group of files with matching or similar hashes.
type DuplicateGroup struct {
	Hash        string    `json:"hash"`                  // phash groups: keeper's phash hex
	HashType    string    `json:"hashType"`              // "sha256" | "phash"
	Size        int64     `json:"size"`                  // exact groups (all files identical)
	MaxDistance int       `json:"maxDistance,omitempty"` // phash groups: max Hamming distance to keeper
	Keeper      *int      `json:"keeper,omitempty"`      // index into Files; nil = quality tie, manual pick
	Files       []DupFile `json:"files"`
}

// DupFile identifies a file in a duplicate group.
type DupFile struct {
	RootID     string `json:"rootId"`
	Path       string `json:"path"`
	Size       int64  `json:"size,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Mtime      int64  `json:"mtime,omitempty"`
	Similarity int    `json:"similarity,omitempty"` // % vs keeper (phash groups only)
}

// FindDuplicates finds files with matching checksums under a directory.
// SHA256 groups cover byte-identical images; CRC32 groups cover videos whose
// CRC32 was parsed from a videotagger-style filename (and which have no
// SHA256). A file never appears in both group types.
func (s *Store) FindDuplicates(rootID, dirPath string) ([]DuplicateGroup, error) {
	prefix := filesystem.DirPrefix(dirPath)

	shaGroups, err := s.findDuplicateGroups(rootID, prefix, "sha256",
		`SELECT c1.sha256, c1.size, c1.rel_path, c1.width, c1.height, c1.mtime
		 FROM classifications c1
		 WHERE c1.root_id = ? AND c1.rel_path LIKE ? AND c1.sha256 IS NOT NULL
		   AND c1.sha256 IN (
		     SELECT sha256 FROM classifications
		     WHERE root_id = ? AND rel_path LIKE ? AND sha256 IS NOT NULL
		     GROUP BY sha256 HAVING COUNT(*) > 1
		   )
		 ORDER BY c1.sha256`)
	if err != nil {
		return nil, err
	}

	crcGroups, err := s.findDuplicateGroups(rootID, prefix, "crc32",
		`SELECT c1.crc32, c1.size, c1.rel_path, c1.width, c1.height, c1.mtime
		 FROM classifications c1
		 WHERE c1.root_id = ? AND c1.rel_path LIKE ?
		   AND c1.crc32 IS NOT NULL AND c1.sha256 IS NULL
		   AND c1.crc32 IN (
		     SELECT crc32 FROM classifications
		     WHERE root_id = ? AND rel_path LIKE ?
		       AND crc32 IS NOT NULL AND sha256 IS NULL
		     GROUP BY crc32 HAVING COUNT(*) > 1
		   )
		 ORDER BY c1.crc32`)
	if err != nil {
		return nil, err
	}

	result := make([]DuplicateGroup, 0, len(shaGroups)+len(crcGroups))
	result = append(result, shaGroups...)
	result = append(result, crcGroups...)
	return result, nil
}

// findDuplicateGroups runs a generic exact-match grouping query. The queryText
// must select (hash, size, rel_path, width, height, mtime) in that order and
// accept (rootID, prefix%, rootID, prefix%) parameters. hashType is the label
// stored on each resulting DuplicateGroup.
func (s *Store) findDuplicateGroups(rootID, prefix, hashType, queryText string) ([]DuplicateGroup, error) {
	rows, err := s.db.Query(queryText, rootID, prefix+"%", rootID, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("finding %s duplicates: %w", hashType, err)
	}
	defer func() { _ = rows.Close() }()

	groups := make(map[string]*DuplicateGroup)
	var order []string
	for rows.Next() {
		var hash, relPath string
		var size, mtime int64
		var widthVal, heightVal sql.NullInt64
		if err := rows.Scan(&hash, &size, &relPath, &widthVal, &heightVal, &mtime); err != nil {
			return nil, err
		}
		g, ok := groups[hash]
		if !ok {
			keeper := 0
			g = &DuplicateGroup{Hash: hash, HashType: hashType, Size: size, Keeper: &keeper}
			groups[hash] = g
			order = append(order, hash)
		}
		g.Files = append(g.Files, DupFile{
			RootID: rootID,
			Path:   relPath,
			Size:   size,
			Width:  int(widthVal.Int64),
			Height: int(heightVal.Int64),
			Mtime:  mtime,
		})
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

// NeedsPHash reports whether a cached classification row exists but has no
// perceptual hash yet (legacy rows from before phash support). Freshness of
// the row itself is already gated by IsStale in the analysis coordinator.
func (s *Store) NeedsPHash(rootID, relPath string) bool {
	var missing bool
	err := s.db.QueryRow(
		`SELECT phash IS NULL FROM classifications WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	).Scan(&missing)
	return err == nil && missing
}

// UpdatePHash backfills the perceptual hash and dimensions of an existing
// classification row without touching tags, mtime, or model version.
func (s *Store) UpdatePHash(rootID, relPath, phash string, width, height int) error {
	_, err := s.db.Exec(
		`UPDATE classifications SET phash = ?, width = ?, height = ? WHERE root_id = ? AND rel_path = ?`,
		nullString(phash), nullInt(width), nullInt(height), rootID, relPath,
	)
	if err != nil {
		return fmt.Errorf("updating phash: %w", err)
	}
	return nil
}

// FindSimilar finds groups of visually similar images under a directory by
// clustering perceptual hashes within maxDist Hamming bits. Byte-identical
// files are collapsed to one representative (those belong to FindDuplicates).
func (s *Store) FindSimilar(rootID, dirPath string, maxDist int) ([]DuplicateGroup, error) {
	prefix := filesystem.DirPrefix(dirPath)

	rows, err := s.db.Query(
		`SELECT rel_path, phash, size, width, height, mtime, sha256
		 FROM classifications
		 WHERE root_id = ? AND rel_path LIKE ? AND phash IS NOT NULL
		 ORDER BY rel_path`,
		rootID, prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("finding similar: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []PHashEntry
	for rows.Next() {
		var relPath, phashHex string
		var sha256Val sql.NullString
		var widthVal, heightVal sql.NullInt64
		var size, mtime int64
		if err := rows.Scan(&relPath, &phashHex, &size, &widthVal, &heightVal, &mtime, &sha256Val); err != nil {
			return nil, err
		}
		h, ok := parsePHash(phashHex)
		if !ok {
			continue
		}
		entries = append(entries, PHashEntry{
			RelPath: relPath,
			PHash:   h,
			Size:    size,
			Width:   int(widthVal.Int64),
			Height:  int(heightVal.Int64),
			Mtime:   mtime,
			SHA256:  sha256Val.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	clusters := groupSimilar(entries, maxDist)
	groups := make([]DuplicateGroup, 0, len(clusters))
	for _, members := range clusters {
		groups = append(groups, buildSimilarGroup(rootID, members))
	}
	return groups, nil
}

// buildSimilarGroup turns one cluster into a DuplicateGroup: files sorted by
// resolution (best first, path as tiebreak), keeper = the highest-resolution
// file unless several share the top resolution (quality tie → nil keeper),
// per-file similarity computed against the best file's hash.
func buildSimilarGroup(rootID string, members []PHashEntry) DuplicateGroup {
	sort.Slice(members, func(i, j int) bool {
		ai, aj := members[i].Width*members[i].Height, members[j].Width*members[j].Height
		if ai != aj {
			return ai > aj
		}
		return members[i].RelPath < members[j].RelPath
	})

	best := members[0]
	g := DuplicateGroup{
		Hash:     fmt.Sprintf("%016x", best.PHash),
		HashType: "phash",
	}
	if best.Width*best.Height != members[1].Width*members[1].Height {
		keeper := 0
		g.Keeper = &keeper
	}

	for _, m := range members {
		d := hammingDistance(best.PHash, m.PHash)
		if d > g.MaxDistance {
			g.MaxDistance = d
		}
		g.Files = append(g.Files, DupFile{
			RootID:     rootID,
			Path:       m.RelPath,
			Size:       m.Size,
			Width:      m.Width,
			Height:     m.Height,
			Mtime:      m.Mtime,
			Similarity: DistanceToSimilarity(d),
		})
	}
	return g
}

// ListPaths returns every stored rel_path under a directory (recursive).
func (s *Store) ListPaths(rootID, dirPath string) ([]string, error) {
	match := newScopedPathMatch(dirPath)
	var rows *sql.Rows
	var err error
	if match.ok {
		rows, err = s.db.Query(
			`SELECT rel_path FROM classifications
			 WHERE root_id = ? AND (
			   (rel_path = ? OR rel_path LIKE ? ESCAPE '\') OR
			   (rel_path = ? OR rel_path LIKE ? ESCAPE '\')
			 )
			 ORDER BY rel_path`,
			rootID, match.first, match.firstPrefix, match.second, match.secondPrefix,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT rel_path FROM classifications WHERE root_id = ? ORDER BY rel_path`,
			rootID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("listing paths: %w", err)
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

// DeletePath removes cached results for a file, or for a whole subtree when
// the path is a directory (prefix match on rel_path).
func (s *Store) DeletePath(rootID, relPath string) error {
	match := newScopedPathMatch(relPath)
	if !match.ok {
		return errors.New("refusing to delete root path")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(
		`DELETE FROM tags
		 WHERE root_id = ? AND (
		   (rel_path = ? OR rel_path LIKE ? ESCAPE '\') OR
		   (rel_path = ? OR rel_path LIKE ? ESCAPE '\')
		 )`,
		rootID, match.first, match.firstPrefix, match.second, match.secondPrefix,
	)
	if err != nil {
		return fmt.Errorf("deleting tags: %w", err)
	}
	_, err = tx.Exec(
		`DELETE FROM classifications
		 WHERE root_id = ? AND (
		   (rel_path = ? OR rel_path LIKE ? ESCAPE '\') OR
		   (rel_path = ? OR rel_path LIKE ? ESCAPE '\')
		 )`,
		rootID, match.first, match.firstPrefix, match.second, match.secondPrefix,
	)
	if err != nil {
		return fmt.Errorf("deleting classifications: %w", err)
	}

	return tx.Commit()
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}
