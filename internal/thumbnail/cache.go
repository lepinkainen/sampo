package thumbnail

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Cache manages thumbnail storage on disk.
type Cache struct {
	baseDir string
}

// NewCache creates a new thumbnail cache.
func NewCache(baseDir string) (*Cache, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}
	return &Cache{baseDir: baseDir}, nil
}

// CacheKey generates a unique key for a file based on path, mtime, and size.
func CacheKey(rootID, relPath string, modTime int64, size int64) string {
	data := fmt.Sprintf("%s:%s:%d:%d", rootID, relPath, modTime, size)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:16])
}

// Path returns the cache file path for a given key.
func (c *Cache) Path(rootID, key string) string {
	dir := filepath.Join(c.baseDir, rootID)
	return filepath.Join(dir, key+".jpg")
}

// Get checks if a cached thumbnail exists and returns its path.
func (c *Cache) Get(rootID, key string) (string, bool) {
	p := c.Path(rootID, key)
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	return "", false
}

// EnsureDir creates the cache subdirectory for a root.
func (c *Cache) EnsureDir(rootID string) error {
	dir := filepath.Join(c.baseDir, rootID)
	return os.MkdirAll(dir, 0o755)
}

// Prune removes cached thumbnails older than maxAge based on file mtime.
// Returns the number of files removed and the first error encountered.
func (c *Cache) Prune(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	var firstErr error

	err := filepath.WalkDir(c.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return nil
			}
			removed++
		}
		return nil
	})
	if err != nil && firstErr == nil {
		firstErr = err
	}

	return removed, firstErr
}
