package metadata

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/lepinkainen/sampo/internal/filesystem"
)

// Store records per-file media dimensions (and video duration) in SQLite so
// directory listings can show resolution without re-decoding images or
// re-probing videos on every request. It is intentionally decoupled from the
// ML stores: it covers every media file the coordinator touches, regardless of
// which analyzers are enabled, and stores only cheaply-obtained geometry.
type Store struct {
	db *sql.DB
}

// Dims holds the stored dimensions and (for videos) duration for a file.
type Dims struct {
	Width    int
	Height   int
	Duration float64 // seconds; 0 for images
}

// NewStore opens or creates the metadata database.
func NewStore(cacheDir string) (*Store, error) {
	dbPath := filepath.Join(cacheDir, "metadata.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("opening metadata db: %w", err)
	}
	// SQLite allows only one writer; a single connection serializes writes
	// in-process instead of failing with SQLITE_BUSY under concurrent Puts.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			root_id    TEXT NOT NULL,
			rel_path   TEXT NOT NULL,
			mtime      INTEGER NOT NULL,
			width      INTEGER NOT NULL,
			height     INTEGER NOT NULL,
			duration   REAL NOT NULL DEFAULT 0,
			scanned_at DATETIME NOT NULL,
			PRIMARY KEY (root_id, rel_path)
		);
		CREATE INDEX IF NOT EXISTS idx_metadata_dir ON metadata(root_id, rel_path);
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating metadata db: %w", err)
	}

	return &Store{db: db}, nil
}

// Put inserts or replaces the stored dimensions for a file.
func (s *Store) Put(rootID, relPath string, mtime int64, d Dims) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO metadata (root_id, rel_path, mtime, width, height, duration, scanned_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
		rootID, relPath, mtime, d.Width, d.Height, d.Duration,
	)
	if err != nil {
		return fmt.Errorf("upserting metadata: %w", err)
	}
	return nil
}

// IsStale reports whether metadata for a file is missing or stale (mtime changed).
func (s *Store) IsStale(rootID, relPath string, mtime int64) bool {
	var storedMtime int64
	err := s.db.QueryRow(
		`SELECT mtime FROM metadata WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	).Scan(&storedMtime)
	if err != nil {
		return true
	}
	return storedMtime != mtime
}

// Get returns the stored dimensions for a single file. ok is false when no row
// exists (including when the file has never been analyzed).
func (s *Store) Get(rootID, relPath string) (d Dims, ok bool, err error) {
	err = s.db.QueryRow(
		`SELECT width, height, duration FROM metadata WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	).Scan(&d.Width, &d.Height, &d.Duration)
	if errors.Is(err, sql.ErrNoRows) {
		return Dims{}, false, nil
	}
	if err != nil {
		return Dims{}, false, fmt.Errorf("getting metadata: %w", err)
	}
	return d, true, nil
}

// GetMany returns a map of relPath -> Dims for the given files in a single
// query per chunk, so callers enriching result lists (e.g. search) avoid one
// query per file. Paths without stored metadata are absent from the map.
func (s *Store) GetMany(rootID string, relPaths []string) (map[string]Dims, error) {
	result := make(map[string]Dims, len(relPaths))
	// Stay well below SQLite's bound-parameter limit per statement.
	const chunkSize = 500
	for start := 0; start < len(relPaths); start += chunkSize {
		chunk := relPaths[start:min(start+chunkSize, len(relPaths))]
		if err := s.getManyChunk(rootID, chunk, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Store) getManyChunk(rootID string, chunk []string, result map[string]Dims) error {
	// The concatenated fragment is only "?" placeholders — every value is bound.
	placeholders := strings.Repeat("?,", len(chunk)-1) + "?"
	args := make([]any, 0, len(chunk)+1)
	args = append(args, rootID)
	for _, p := range chunk {
		args = append(args, p)
	}
	rows, err := s.db.Query(
		`SELECT rel_path, width, height, duration FROM metadata
		 WHERE root_id = ? AND rel_path IN (`+placeholders+`)`, // #nosec G202 -- placeholders only
		args...,
	)
	if err != nil {
		return fmt.Errorf("getting metadata batch: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var relPath string
		var d Dims
		if err := rows.Scan(&relPath, &d.Width, &d.Height, &d.Duration); err != nil {
			return err
		}
		result[relPath] = d
	}
	return rows.Err()
}

// GetDirDimensions returns a map of relPath -> Dims for the direct children of
// dirPath. Nested descendants are excluded so callers can merge results onto a
// single-level directory listing without extra filtering.
func (s *Store) GetDirDimensions(rootID, dirPath string) (map[string]Dims, error) {
	prefix := filesystem.DirPrefix(dirPath)

	rows, err := s.db.Query(
		`SELECT rel_path, width, height, duration FROM metadata
		 WHERE root_id = ? AND rel_path LIKE ?`,
		rootID, prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("getting dir dimensions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]Dims)
	for rows.Next() {
		var relPath string
		var d Dims
		if err := rows.Scan(&relPath, &d.Width, &d.Height, &d.Duration); err != nil {
			return nil, err
		}
		if filesystem.IsDirectChild(relPath, prefix) {
			result[relPath] = d
		}
	}
	return result, rows.Err()
}

// ListPaths returns stored rel_paths under relPath (prefix match), or all paths
// for the root when relPath is empty. Used by the coordinator's prune pass to
// drop metadata for files that no longer exist on disk.
func (s *Store) ListPaths(rootID, relPath string) ([]string, error) {
	var rows *sql.Rows
	var err error
	if relPath == "" {
		rows, err = s.db.Query(`SELECT rel_path FROM metadata WHERE root_id = ? ORDER BY rel_path`, rootID)
	} else {
		rows, err = s.db.Query(`SELECT rel_path FROM metadata WHERE root_id = ? AND rel_path LIKE ? ORDER BY rel_path`, rootID, filesystem.DirPrefix(relPath)+"%")
	}
	if err != nil {
		return nil, fmt.Errorf("listing paths: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// DeletePath removes stored metadata for a file, or for a whole subtree when
// relPath is a directory (prefix match on rel_path). Refuses the root path.
func (s *Store) DeletePath(rootID, relPath string) error {
	if strings.Trim(relPath, "/") == "" {
		return errors.New("refusing to delete root path")
	}
	_, err := s.db.Exec(
		`DELETE FROM metadata WHERE root_id = ? AND (rel_path = ? OR rel_path LIKE ?)`,
		rootID, relPath, filesystem.DirPrefix(relPath)+"%",
	)
	if err != nil {
		return fmt.Errorf("deleting metadata: %w", err)
	}
	return nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }
