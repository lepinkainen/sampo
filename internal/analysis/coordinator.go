package analysis

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/lepinkainen/filemanager/internal/classification"
	"github.com/lepinkainen/filemanager/internal/detection"
	"github.com/lepinkainen/filemanager/internal/videoframe"
)

// Coordinator schedules low-priority background analysis triggered by browsing.
type Coordinator struct {
	detectionStore *detection.Store
	detector       *detection.Detector
	classStore     *classification.Store
	classifier     *classification.Classifier
	frameDir       string
	includeVideos  bool
	logger         *slog.Logger

	jobs chan job

	mu      sync.Mutex
	pending map[string]struct{}
	active  atomic.Int32
}

type job struct {
	key          string
	rootID       string
	relPath      string
	fullPath     string
	mediaType    string
	mtime        int64
	size         int64
	needDetect   bool
	needClassify bool
}

// Status reports current browse-analysis activity.
type Status struct {
	Pending int  `json:"pending"`
	Queued  int  `json:"queued"`
	Active  int  `json:"active"`
	Running bool `json:"running"`
}

// NewCoordinator creates a browse-triggered analysis coordinator.
func NewCoordinator(
	detectionStore *detection.Store,
	detector *detection.Detector,
	classStore *classification.Store,
	classifier *classification.Classifier,
	frameDir string,
	workers int,
	queueSize int,
	includeVideos bool,
	logger *slog.Logger,
) *Coordinator {
	if workers < 1 {
		workers = 1
	}
	if queueSize < 1 {
		queueSize = 1
	}

	c := &Coordinator{
		detectionStore: detectionStore,
		detector:       detector,
		classStore:     classStore,
		classifier:     classifier,
		frameDir:       frameDir,
		includeVideos:  includeVideos,
		logger:         logger,
		jobs:           make(chan job, queueSize),
		pending:        make(map[string]struct{}),
	}

	for i := 0; i < workers; i++ {
		go c.worker()
	}

	return c
}

// Enqueue schedules background analysis if at least one enabled model needs fresh data.
// Returns true if a job was queued.
func (c *Coordinator) Enqueue(rootID, relPath, fullPath, mediaType string, mtime, size int64) bool {
	if c == nil {
		return false
	}
	if mediaType != "image" && (!c.includeVideos || mediaType != "video") {
		return false
	}

	needDetect := c.detectionStore != nil && c.detector != nil && c.detectionStore.IsStale(rootID, relPath, mtime, size)
	needClassify := c.classStore != nil && c.classifier != nil && c.classStore.IsStale(rootID, relPath, mtime, size)
	if !needDetect && !needClassify {
		return false
	}

	key := fmt.Sprintf("%s|%s|%d|%d", rootID, relPath, mtime, size)

	c.mu.Lock()
	if _, exists := c.pending[key]; exists {
		c.mu.Unlock()
		return false
	}
	c.pending[key] = struct{}{}
	c.mu.Unlock()

	j := job{
		key:          key,
		rootID:       rootID,
		relPath:      relPath,
		fullPath:     fullPath,
		mediaType:    mediaType,
		mtime:        mtime,
		size:         size,
		needDetect:   needDetect,
		needClassify: needClassify,
	}

	select {
	case c.jobs <- j:
		return true
	default:
		c.mu.Lock()
		delete(c.pending, key)
		c.mu.Unlock()
		c.logger.Debug("browse analysis queue full; dropping job", "path", relPath)
		return false
	}
}

func (c *Coordinator) worker() {
	for j := range c.jobs {
		c.active.Add(1)
		c.process(j)
		c.active.Add(-1)
		c.mu.Lock()
		delete(c.pending, j.key)
		c.mu.Unlock()
	}
}

// Status returns current browse-analysis activity.
func (c *Coordinator) Status() Status {
	if c == nil {
		return Status{}
	}

	c.mu.Lock()
	pending := len(c.pending)
	c.mu.Unlock()

	active := int(c.active.Load())
	queued := pending - active
	if queued < 0 {
		queued = 0
	}
	return Status{
		Pending: pending,
		Queued:  queued,
		Active:  active,
		Running: pending > 0 || active > 0,
	}
}

func (c *Coordinator) process(j job) {
	analyzePath := j.fullPath
	var cleanup func() error

	if j.mediaType == "video" {
		framePath, frameCleanup, err := videoframe.ExtractFrame(c.frameDir, j.fullPath)
		if err != nil {
			c.logger.Warn("browse analysis video frame extraction failed", "path", j.relPath, "error", err)
			return
		}
		analyzePath = framePath
		cleanup = frameCleanup
	}

	if cleanup != nil {
		defer func() {
			if err := cleanup(); err != nil {
				c.logger.Warn("failed to remove browse analysis temp frame", "path", analyzePath, "error", err)
			}
		}()
	}

	if j.needDetect {
		result, err := c.detector.Detect(analyzePath, j.rootID, j.relPath, j.mtime, j.size)
		if err != nil {
			c.logger.Warn("browse analysis detection failed", "path", j.relPath, "error", err)
		} else if err := c.detectionStore.Put(result); err != nil {
			c.logger.Warn("storing browse analysis detection result", "path", j.relPath, "error", err)
		}
	}

	if j.needClassify {
		result, err := c.classifier.Classify(analyzePath, j.rootID, j.relPath, j.mtime, j.size)
		if err != nil {
			c.logger.Warn("browse analysis classification failed", "path", j.relPath, "error", err)
		} else if err := c.classStore.Put(result); err != nil {
			c.logger.Warn("storing browse analysis classification result", "path", j.relPath, "error", err)
		}
	}
}
