package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/lepinkainen/sampo/internal/classification"
	"github.com/lepinkainen/sampo/internal/detection"
	"github.com/lepinkainen/sampo/internal/videoframe"
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

// videoAnalysisFrames is the number of frames extracted for video ML analysis,
// matching the thumbnail grid's 4-frame sampling for consistent coverage.
const videoAnalysisFrames = 4

func (c *Coordinator) process(j job) {
	if j.mediaType == "video" {
		c.processVideo(j)
		return
	}
	c.processImage(j, j.fullPath)
}

func (c *Coordinator) processImage(j job, analyzePath string) {
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

func (c *Coordinator) processVideo(j job) {
	framePaths, cleanup, err := videoframe.ExtractFrames(context.Background(), c.frameDir, j.fullPath, videoAnalysisFrames)
	if err != nil {
		c.logger.Warn("browse analysis video frame extraction failed", "path", j.relPath, "error", err)
		return
	}
	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil {
			c.logger.Warn("failed to remove browse analysis temp frames", "path", j.relPath, "error", cleanupErr)
		}
	}()

	if j.needDetect {
		var results []*detection.Result
		for _, fp := range framePaths {
			result, detectErr := c.detector.Detect(fp, j.rootID, j.relPath, j.mtime, j.size)
			if detectErr != nil {
				c.logger.Debug("browse analysis detection failed for frame", "path", j.relPath, "frame", fp, "error", detectErr)
				continue
			}
			results = append(results, result)
		}
		if agg := aggregateDetections(results); agg != nil {
			if putErr := c.detectionStore.Put(agg); putErr != nil {
				c.logger.Warn("storing browse analysis detection result", "path", j.relPath, "error", putErr)
			}
		}
	}

	if j.needClassify {
		var results []*classification.Result
		for _, fp := range framePaths {
			result, classErr := c.classifier.Classify(fp, j.rootID, j.relPath, j.mtime, j.size)
			if classErr != nil {
				c.logger.Debug("browse analysis classification failed for frame", "path", j.relPath, "frame", fp, "error", classErr)
				continue
			}
			results = append(results, result)
		}
		if agg := aggregateClassifications(results); agg != nil {
			// Clear file hashes — they're from temporary frames, not the original video.
			agg.SHA256 = ""
			agg.CRC32 = ""
			if putErr := c.classStore.Put(agg); putErr != nil {
				c.logger.Warn("storing browse analysis classification result", "path", j.relPath, "error", putErr)
			}
		}
	}
}

// aggregateDetections merges detection results from multiple frames.
// HasPerson is true if any frame detected a person; Confidence is the maximum.
func aggregateDetections(results []*detection.Result) *detection.Result {
	if len(results) == 0 {
		return nil
	}
	agg := *results[0]
	for _, r := range results[1:] {
		if r.HasPerson {
			agg.HasPerson = true
		}
		if r.Confidence > agg.Confidence {
			agg.Confidence = r.Confidence
		}
	}
	return &agg
}

// aggregateClassifications merges classification results from multiple frames.
// Tags are unioned, keeping the highest score per label.
func aggregateClassifications(results []*classification.Result) *classification.Result {
	if len(results) == 0 {
		return nil
	}
	agg := *results[0]
	best := make(map[string]float32)
	for _, r := range results {
		for _, tag := range r.Tags {
			if tag.Score > best[tag.Label] {
				best[tag.Label] = tag.Score
			}
		}
	}
	agg.Tags = make([]classification.TagScore, 0, len(best))
	for label, score := range best {
		agg.Tags = append(agg.Tags, classification.TagScore{Label: label, Score: score})
	}
	return &agg
}
