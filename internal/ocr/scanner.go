package ocr

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

// ScanStatus reports the progress of a background OCR scan.
type ScanStatus = scanstatus.Snapshot

// Scanner manages background OCR scanning.
type Scanner struct {
	store      *Store
	recognizer *Recognizer
	roots      *filesystem.RootManager
	frameDir   string
	workers    int
	logger     *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	status atomic.Pointer[scanstatus.State]
}

// NewScanner creates a background OCR scanner.
func NewScanner(store *Store, recognizer *Recognizer, roots *filesystem.RootManager, frameDir string, workers int, logger *slog.Logger) *Scanner {
	s := &Scanner{
		store:      store,
		recognizer: recognizer,
		roots:      roots,
		frameDir:   frameDir,
		workers:    workers,
		logger:     logger,
	}
	s.status.Store(scanstatus.NewIdle())
	return s
}

// ScanDirectory starts a background OCR scan of images in a directory.
// Returns immediately. Only one scan runs at a time.
//
// When force is false, only the immediate directory level is scanned and files
// with fresh cached results are skipped. When force is true, the scan recurses
// and re-OCRs every image, replacing existing results.
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

	status := scanstatus.New(rootID, relPath, int64(len(files)))
	s.status.Store(status)

	if len(files) == 0 {
		status.Complete()
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
					s.processItem(ctx, status, item)
				}
			}()
		}
		wg.Wait()

		status.Complete()
	}()

	return nil
}

// collectFiles gathers the images/videos to OCR. When force is true it walks
// recursively and includes every file; otherwise it lists only the immediate
// directory level and skips files with fresh cached results.
func (s *Scanner) collectFiles(rootID, rootPath, fullPath, relPath string, force bool) ([]scanItem, error) {
	var files []scanItem

	add := func(name string, info os.FileInfo) {
		mediaType := filesystem.DetectMediaType(name)
		if mediaType != "image" && mediaType != "video" {
			return
		}
		entryRelPath := filepath.Join(relPath, strings.TrimPrefix(name, fullPath))
		entryRelPath = filepath.Clean(entryRelPath)
		if !force && !s.store.IsStale(rootID, entryRelPath, info.ModTime().Unix(), info.Size(), s.recognizer.ModelVersion()) {
			return
		}
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

type scanItem struct {
	rootID    string
	relPath   string
	fullPath  string
	mtime     int64
	size      int64
	mediaType string
}

func (s *Scanner) processItem(ctx context.Context, status *scanstatus.State, item scanItem) {
	ocrPath := item.fullPath
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
		ocrPath = framePath
	}

	result, err := s.recognizer.Recognize(ctx, nil, ocrPath, item.rootID, item.relPath, item.mtime, item.size)
	if err != nil {
		s.logger.Error("ocr failed", "path", item.relPath, "error", err)
		status.RecordError()
		return
	}

	if err := s.store.Put(result); err != nil {
		s.logger.Error("storing ocr result", "path", item.relPath, "error", err)
		status.AddError(1)
	}

	status.AddCompleted(1)

	s.logger.Debug("ocr complete", "path", item.relPath, "chars", len(result.Text))
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
