package analysis

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lepinkainen/sampo/internal/filesystem"
)

// ScanStatus reports the progress of a unified background analysis scan.
type ScanStatus struct {
	Running   bool   `json:"running"`
	RootID    string `json:"rootId,omitempty"`
	Path      string `json:"path,omitempty"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Errors    int64  `json:"errors"`
}

// Scanner runs a unified directory scan that loads each file once and runs every
// enabled analyzer (detection, classification, OCR) on it, via the Coordinator.
// This is what the "re-analyze" action triggers — one pass, all classifiers.
type Scanner struct {
	coord   *Coordinator
	roots   *filesystem.RootManager
	workers int
	logger  *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	status atomic.Pointer[ScanStatus]
}

// NewScanner creates a unified analysis scanner.
func NewScanner(coord *Coordinator, roots *filesystem.RootManager, workers int, logger *slog.Logger) *Scanner {
	if workers < 1 {
		workers = 1
	}
	s := &Scanner{
		coord:   coord,
		roots:   roots,
		workers: workers,
		logger:  logger,
	}
	s.status.Store(&ScanStatus{})
	return s
}

// ScanDirectory starts a background unified analysis scan. Returns immediately.
// Only one scan runs at a time.
//
// When force is false, only the immediate directory level is scanned and each
// analyzer skips files whose cached result is still fresh. When force is true,
// the scan recurses into subdirectories and re-runs every analyzer on every
// file, replacing existing results.
func (s *Scanner) ScanDirectory(rootID, relPath string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	root, err := s.roots.Get(rootID)
	if err != nil {
		return err
	}

	fullPath, err := s.roots.ResolvePath(rootID, relPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	files, err := s.collectFiles(rootID, root.Path, fullPath, relPath, force)
	if err != nil {
		cancel()
		return err
	}

	status := &ScanStatus{
		Running: true,
		RootID:  rootID,
		Path:    relPath,
		Total:   int64(len(files)),
	}
	s.status.Store(status)

	if len(files) == 0 {
		status.Running = false
		s.status.Store(status)
		cancel()
		return nil
	}

	ch := make(chan scanItem, len(files))
	for _, f := range files {
		ch <- f
	}
	close(ch)

	go func() {
		var wg sync.WaitGroup
		for range s.workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range ch {
					if ctx.Err() != nil {
						return
					}
					s.coord.Analyze(ctx, item.rootID, item.relPath, item.fullPath, item.mediaType, item.mtime, item.size, force)
					current := s.status.Load()
					atomic.AddInt64(&current.Completed, 1)
				}
			}()
		}
		wg.Wait()

		current := s.status.Load()
		done := *current
		done.Running = false
		s.status.Store(&done)
	}()

	return nil
}

type scanItem struct {
	rootID    string
	relPath   string
	fullPath  string
	mtime     int64
	size      int64
	mediaType string
}

// collectFiles gathers media to analyze. When force is true it walks recursively
// and includes every file; otherwise it lists only the immediate directory level.
func (s *Scanner) collectFiles(rootID, rootPath, fullPath, relPath string, force bool) ([]scanItem, error) {
	var files []scanItem

	add := func(name string, info os.FileInfo) {
		mediaType := filesystem.DetectMediaType(name)
		if mediaType != "image" && mediaType != "video" {
			return
		}
		entryRelPath := filepath.Join(relPath, strings.TrimPrefix(name, fullPath))
		entryRelPath = filepath.Clean(entryRelPath)
		files = append(files, scanItem{
			rootID:    rootID,
			relPath:   entryRelPath,
			fullPath:  filepath.Join(rootPath, entryRelPath),
			mtime:     info.ModTime().Unix(),
			size:      info.Size(),
			mediaType: mediaType,
		})
	}

	if force {
		err := filepath.WalkDir(fullPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() && path != fullPath {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			info, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}
			add(path, info)
			return nil
		})
		return files, err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		add(filepath.Join(fullPath, e.Name()), info)
	}
	return files, nil
}

// Status returns the current scan status.
func (s *Scanner) Status() ScanStatus {
	return *s.status.Load()
}

// Stop cancels any running scan.
func (s *Scanner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}
