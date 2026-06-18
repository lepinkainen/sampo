package detection

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/scanstatus"
	"github.com/lepinkainen/sampo/internal/videoframe"
)

// ScanStatus reports the progress of a background scan.
type ScanStatus = scanstatus.Snapshot

// Scanner manages background detection scanning.
type Scanner struct {
	store    *Store
	detector *Detector
	roots    *filesystem.RootManager
	frameDir string
	workers  int
	logger   *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	status atomic.Pointer[scanstatus.State]
}

// NewScanner creates a background scanner.
func NewScanner(store *Store, detector *Detector, roots *filesystem.RootManager, frameDir string, workers int, logger *slog.Logger) *Scanner {
	s := &Scanner{
		store:    store,
		detector: detector,
		roots:    roots,
		frameDir: frameDir,
		workers:  workers,
		logger:   logger,
	}
	s.status.Store(scanstatus.NewIdle())
	return s
}

// ScanDirectory starts a background scan of images in a directory.
// Returns immediately. Only one scan runs at a time.
func (s *Scanner) ScanDirectory(rootID, relPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel any existing scan
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

	// Collect image files
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

		// Skip if already scanned and not stale
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

	status := scanstatus.New(rootID, relPath, int64(len(files)))
	s.status.Store(status)

	if len(files) == 0 {
		status.Complete()
		cancel()
		return nil
	}

	// Launch worker pool
	ch := make(chan scanItem, len(files))
	for _, f := range files {
		ch <- f
	}
	close(ch)

	go func() {
		var wg sync.WaitGroup
		for i := 0; i < s.workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range ch {
					if ctx.Err() != nil {
						return
					}
					s.processItem(ctx, status, item)
				}
			}()
		}
		wg.Wait()

		// Mark scan as complete
		status.Complete()
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

func (s *Scanner) processItem(ctx context.Context, status *scanstatus.State, item scanItem) {
	detectPath := item.fullPath
	if item.mediaType == "video" {
		framePath, cleanup, err := videoframe.ExtractFrame(ctx, s.frameDir, item.fullPath)
		if err != nil {
			s.logger.Error("video frame extraction failed", "path", item.relPath, "error", err)
			status.RecordError()
			return
		}
		defer func() {
			if err := cleanup(); err != nil {
				s.logger.Warn("failed to remove temp video frame", "path", framePath, "error", err)
			}
		}()
		detectPath = framePath
	}

	result, err := s.detector.Detect(detectPath, item.rootID, item.relPath, item.mtime, item.size)
	if err != nil {
		s.logger.Error("detection failed", "path", item.relPath, "error", err)
		// Increment errors
		status.RecordError()
		return
	}

	if err := s.store.Put(result); err != nil {
		s.logger.Error("storing detection result", "path", item.relPath, "error", err)
		status.AddError(1)
	}

	status.AddCompleted(1)

	s.logger.Debug("scanned", "path", item.relPath, "hasPerson", result.HasPerson, "confidence", result.Confidence)
}

// Status returns the current scan status.
func (s *Scanner) Status() ScanStatus {
	return s.status.Load().Snapshot()
}

// Stop cancels any running scan.
func (s *Scanner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}
