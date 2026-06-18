package detection

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Result holds the result of a person detection scan.
type Result struct {
	RootID     string    `json:"rootId"`
	RelPath    string    `json:"relPath"`
	Mtime      int64     `json:"mtime"`
	Size       int64     `json:"size"`
	HasPerson  bool      `json:"hasPerson"`
	Confidence float64   `json:"confidence"`
	ModelVer   string    `json:"modelVer"`
	ScannedAt  time.Time `json:"scannedAt"`
}

// Store manages detection results in SQLite.
type Store struct {
	db *sql.DB
}

// NewStore opens or creates the detection database.
func NewStore(cacheDir string) (*Store, error) {
	dbPath := filepath.Join(cacheDir, "detection.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening detection db: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS detections (
			root_id    TEXT NOT NULL,
			rel_path   TEXT NOT NULL,
			mtime      INTEGER NOT NULL,
			size       INTEGER NOT NULL,
			has_person BOOLEAN NOT NULL,
			confidence REAL,
			model_ver  TEXT NOT NULL,
			scanned_at DATETIME NOT NULL,
			PRIMARY KEY (root_id, rel_path)
		);
		CREATE INDEX IF NOT EXISTS idx_filter ON detections(root_id, has_person);
		CREATE INDEX IF NOT EXISTS idx_dir_path ON detections(root_id, rel_path);
	`)
	if err != nil {
		return fmt.Errorf("migrating detection db: %w", err)
	}
	return nil
}

// Get retrieves a detection result by root and path.
func (s *Store) Get(rootID, relPath string) (*Result, error) {
	row := s.db.QueryRow(
		`SELECT root_id, rel_path, mtime, size, has_person, confidence, model_ver, scanned_at
		 FROM detections WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	)

	var r Result
	err := row.Scan(&r.RootID, &r.RelPath, &r.Mtime, &r.Size, &r.HasPerson, &r.Confidence, &r.ModelVer, &r.ScannedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying detection: %w", err)
	}
	return &r, nil
}

// Put inserts or updates a detection result.
func (s *Store) Put(result *Result) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO detections (root_id, rel_path, mtime, size, has_person, confidence, model_ver, scanned_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		result.RootID, result.RelPath, result.Mtime, result.Size,
		result.HasPerson, result.Confidence, result.ModelVer, result.ScannedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting detection: %w", err)
	}
	return nil
}

// IsStale checks if a file has changed since its last scan.
func (s *Store) IsStale(rootID, relPath string, mtime int64, size int64) bool {
	r, err := s.Get(rootID, relPath)
	if err != nil || r == nil {
		return true
	}
	return r.Mtime != mtime || r.Size != size
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

// DirStatus returns the count of scanned and total-with-person files in a directory.
type DirStatus struct {
	Scanned   int `json:"scanned"`
	HasPerson int `json:"hasPerson"`
}

// GetDirStatus returns scan status for a directory.
func (s *Store) GetDirStatus(rootID, dirPath string) (*DirStatus, error) {
	prefix := dirPrefix(dirPath)

	row := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN has_person THEN 1 ELSE 0 END), 0)
		 FROM detections WHERE root_id = ? AND rel_path LIKE ?`,
		rootID, prefix+"%",
	)

	var status DirStatus
	if err := row.Scan(&status.Scanned, &status.HasPerson); err != nil {
		return nil, fmt.Errorf("getting dir status: %w", err)
	}
	return &status, nil
}

// GetDirDetections returns a map of relPath -> hasPerson for all scanned direct children of a directory.
func (s *Store) GetDirDetections(rootID, dirPath string) (map[string]bool, error) {
	prefix := dirPrefix(dirPath)

	rows, err := s.db.Query(
		`SELECT rel_path, has_person FROM detections
		 WHERE root_id = ? AND rel_path LIKE ?`,
		rootID, prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("getting dir detections: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]bool)
	for rows.Next() {
		var relPath string
		var hasPerson bool
		if err := rows.Scan(&relPath, &hasPerson); err != nil {
			return nil, err
		}
		if isDirectChild(relPath, prefix) {
			result[relPath] = hasPerson
		}
	}
	return result, rows.Err()
}

// GetDetection returns whether a person was detected for a single file.
func (s *Store) GetDetection(rootID, relPath string) (bool, error) {
	var hasPerson bool
	err := s.db.QueryRow(
		`SELECT has_person FROM detections WHERE root_id = ? AND rel_path = ?`,
		rootID, relPath,
	).Scan(&hasPerson)
	if err != nil {
		return false, err
	}
	return hasPerson, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}
