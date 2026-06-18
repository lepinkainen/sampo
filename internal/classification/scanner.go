package classification

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/videoframe"
)

// ScanStatus reports the progress of a background classification scan.
type ScanStatus struct {
	Running   bool   `json:"running"`
	RootID    string `json:"rootId,omitempty"`
	Path      string `json:"path,omitempty"`
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Errors    int64  `json:"errors"`
}

// Scanner manages background classification scanning.
type Scanner struct {
	store      *Store
	classifier *Classifier
	roots      *filesystem.RootManager
	frameDir   string
	workers    int
	logger     *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	status atomic.Pointer[ScanStatus]
}

// NewScanner creates a background classification scanner.
func NewScanner(store *Store, classifier *Classifier, roots *filesystem.RootManager, frameDir string, workers int, logger *slog.Logger) *Scanner {
	s := &Scanner{
		store:      store,
		classifier: classifier,
		roots:      roots,
		frameDir:   frameDir,
		workers:    workers,
		logger:     logger,
	}
	s.status.Store(&ScanStatus{})
	return s
}

// ScanDirectory starts a background classification scan of images in a directory.
// Returns immediately. Only one scan runs at a time.
func (s *Scanner) ScanDirectory(rootID, relPath string) error {
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

	var files []scanItem
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		cancel()
		return err
	}

	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		mediaType := filesystem.DetectMediaType(e.Name())
		if mediaType != "image" && mediaType != "video" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		entryRelPath := filepath.Join(relPath, e.Name())
		entryFullPath := filepath.Join(root.Path, entryRelPath)

		if !s.store.IsStale(rootID, entryRelPath, info.ModTime().Unix(), info.Size()) {
			continue
		}

		files = append(files, scanItem{
			rootID:    rootID,
			relPath:   entryRelPath,
			fullPath:  entryFullPath,
			mtime:     info.ModTime().Unix(),
			size:      info.Size(),
			mediaType: mediaType,
		})
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
					s.processItem(item)
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

func (s *Scanner) processItem(item scanItem) {
	classifyPath := item.fullPath
	if item.mediaType == "video" {
		framePath, cleanup, err := videoframe.ExtractFrame(s.frameDir, item.fullPath)
		if err != nil {
			s.logger.Error("video frame extraction failed", "path", item.relPath, "error", err)
			current := s.status.Load()
			atomic.AddInt64(&current.Errors, 1)
			atomic.AddInt64(&current.Completed, 1)
			return
		}
		defer func() {
			if err := cleanup(); err != nil {
				s.logger.Warn("failed to remove temp video frame", "path", framePath, "error", err)
			}
		}()
		classifyPath = framePath
	}

	result, err := s.classifier.Classify(classifyPath, item.rootID, item.relPath, item.mtime, item.size)
	if err != nil {
		s.logger.Error("classification failed", "path", item.relPath, "error", err)
		current := s.status.Load()
		atomic.AddInt64(&current.Errors, 1)
		atomic.AddInt64(&current.Completed, 1)
		return
	}

	if err := s.store.Put(result); err != nil {
		s.logger.Error("storing classification result", "path", item.relPath, "error", err)
		current := s.status.Load()
		atomic.AddInt64(&current.Errors, 1)
	}

	current := s.status.Load()
	atomic.AddInt64(&current.Completed, 1)

	s.logger.Debug("classified", "path", item.relPath, "tags", len(result.Tags))
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
