package analysis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"image"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/disintegration/imaging"
	"github.com/lepinkainen/sampo/internal/classification"
	"github.com/lepinkainen/sampo/internal/detection"
	"github.com/lepinkainen/sampo/internal/ocr"
	"github.com/lepinkainen/sampo/internal/videoframe"
)

// Coordinator schedules low-priority background analysis triggered by browsing.
//
// It is the single "load once, run every analyzer" path: a job opens the file
// (or extracts video frames) once and runs detection, classification, and OCR
// on it. Both the browse queue (Enqueue) and the bulk Scanner funnel through
// process(), so adding an analyzer here lights it up everywhere.
type Coordinator struct {
	detectionStore *detection.Store
	detector       *detection.Detector
	classStore     *classification.Store
	classifier     *classification.Classifier
	ocrStore       *ocr.Store
	recognizer     *ocr.Recognizer
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
	needOCR      bool
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
	ocrStore *ocr.Store,
	recognizer *ocr.Recognizer,
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
		ocrStore:       ocrStore,
		recognizer:     recognizer,
		frameDir:       frameDir,
		includeVideos:  includeVideos,
		logger:         logger,
		jobs:           make(chan job, queueSize),
		pending:        make(map[string]struct{}),
	}

	for range workers {
		go c.worker()
	}

	return c
}

// needs reports which enabled analyzers must run for a file. When force is true,
// every enabled analyzer runs; otherwise only those with stale cached results.
func (c *Coordinator) needs(rootID, relPath string, mtime, size int64, force bool) (det, cls, ocrN bool) {
	if c.detectionStore != nil && c.detector != nil {
		det = force || c.detectionStore.IsStale(rootID, relPath, mtime, size)
	}
	if c.classStore != nil && c.classifier != nil {
		cls = force || c.classStore.IsStale(rootID, relPath, mtime, size, c.classifier.ModelVersion())
	}
	if c.ocrStore != nil && c.recognizer != nil {
		ocrN = force || c.ocrStore.IsStale(rootID, relPath, mtime, size, c.recognizer.ModelVersion())
	}
	return det, cls, ocrN
}

// Analyze runs all needed analyzers for a single file synchronously, loading the
// media (or extracting video frames) once. Used by the bulk Scanner.
func (c *Coordinator) Analyze(ctx context.Context, rootID, relPath, fullPath, mediaType string, mtime, size int64, force bool) {
	if c == nil {
		return
	}
	det, cls, ocrN := c.needs(rootID, relPath, mtime, size, force)
	if !det && !cls && !ocrN {
		return
	}
	c.process(ctx, job{
		rootID:       rootID,
		relPath:      relPath,
		fullPath:     fullPath,
		mediaType:    mediaType,
		mtime:        mtime,
		size:         size,
		needDetect:   det,
		needClassify: cls,
		needOCR:      ocrN,
	})
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

	needDetect, needClassify, needOCR := c.needs(rootID, relPath, mtime, size, false)
	if !needDetect && !needClassify && !needOCR {
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
		needOCR:      needOCR,
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
		c.process(context.Background(), j)
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

func (c *Coordinator) process(ctx context.Context, j job) {
	if j.mediaType == "video" {
		c.processVideo(ctx, j)
		return
	}
	c.processImage(ctx, j, j.fullPath)
}

// loadImage reads and decodes a file once, also returning its SHA256/CRC32 so
// every analyzer shares a single read + decode + hash. The hash format matches
// classification.computeFileHashes so duplicate detection stays consistent.
func loadImage(path string) (img image.Image, sha256Hex, crc32Hex string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", "", err
	}
	img, err = imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", "", err
	}
	sum := sha256.Sum256(data)
	return img,
		hex.EncodeToString(sum[:]),
		strings.ToUpper(fmt.Sprintf("%08X", crc32.ChecksumIEEE(data))),
		nil
}

func (c *Coordinator) processImage(ctx context.Context, j job, analyzePath string) {
	// Load once, share across every analyzer (the "same byte-level file").
	img, sha256Hex, crc32Hex, err := loadImage(analyzePath)
	if err != nil {
		c.logger.Warn("browse analysis image load failed", "path", j.relPath, "error", err)
		return
	}

	if j.needDetect {
		result, detErr := c.detector.DetectImage(img, j.rootID, j.relPath, j.mtime, j.size)
		if detErr != nil {
			c.logger.Warn("browse analysis detection failed", "path", j.relPath, "error", detErr)
		} else if putErr := c.detectionStore.Put(result); putErr != nil {
			c.logger.Warn("storing browse analysis detection result", "path", j.relPath, "error", putErr)
		}
	}

	if j.needClassify {
		result, clsErr := c.classifier.ClassifyImage(img, sha256Hex, crc32Hex, j.rootID, j.relPath, j.mtime, j.size)
		if clsErr != nil {
			c.logger.Warn("browse analysis classification failed", "path", j.relPath, "error", clsErr)
		} else if putErr := c.classStore.Put(result); putErr != nil {
			c.logger.Warn("storing browse analysis classification result", "path", j.relPath, "error", putErr)
		}
	}

	if j.needOCR {
		// Pass the shared decode; the macOS subprocess backend ignores it and
		// re-reads analyzePath, the in-process backend reuses it.
		result, ocrErr := c.recognizer.Recognize(ctx, img, analyzePath, j.rootID, j.relPath, j.mtime, j.size)
		if ocrErr != nil {
			c.logger.Warn("browse analysis OCR failed", "path", j.relPath, "error", ocrErr)
		} else if putErr := c.ocrStore.Put(result); putErr != nil {
			c.logger.Warn("storing browse analysis OCR result", "path", j.relPath, "error", putErr)
		}
	}
}

func (c *Coordinator) processVideo(ctx context.Context, j job) {
	framePaths, cleanup, err := videoframe.ExtractFrames(ctx, c.frameDir, j.fullPath, videoAnalysisFrames)
	if err != nil {
		c.logger.Warn("browse analysis video frame extraction failed", "path", j.relPath, "error", err)
		return
	}
	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil {
			c.logger.Warn("failed to remove browse analysis temp frames", "path", j.relPath, "error", cleanupErr)
		}
	}()

	// Decode each frame once and run every analyzer on it, then aggregate.
	var detResults []*detection.Result
	var clsResults []*classification.Result
	var ocrResults []*ocr.Result
	for _, fp := range framePaths {
		img, _, _, loadErr := loadImage(fp)
		if loadErr != nil {
			c.logger.Debug("browse analysis frame load failed", "path", j.relPath, "frame", fp, "error", loadErr)
			continue
		}

		if j.needDetect {
			if result, detErr := c.detector.DetectImage(img, j.rootID, j.relPath, j.mtime, j.size); detErr != nil {
				c.logger.Debug("browse analysis detection failed for frame", "path", j.relPath, "frame", fp, "error", detErr)
			} else {
				detResults = append(detResults, result)
			}
		}
		if j.needClassify {
			// Hashes left empty — frames are temporary, not the original video.
			if result, clsErr := c.classifier.ClassifyImage(img, "", "", j.rootID, j.relPath, j.mtime, j.size); clsErr != nil {
				c.logger.Debug("browse analysis classification failed for frame", "path", j.relPath, "frame", fp, "error", clsErr)
			} else {
				clsResults = append(clsResults, result)
			}
		}
		if j.needOCR {
			if result, ocrErr := c.recognizer.Recognize(ctx, img, fp, j.rootID, j.relPath, j.mtime, j.size); ocrErr != nil {
				c.logger.Debug("browse analysis OCR failed for frame", "path", j.relPath, "frame", fp, "error", ocrErr)
			} else {
				ocrResults = append(ocrResults, result)
			}
		}
	}

	if j.needDetect {
		if agg := aggregateDetections(detResults); agg != nil {
			if putErr := c.detectionStore.Put(agg); putErr != nil {
				c.logger.Warn("storing browse analysis detection result", "path", j.relPath, "error", putErr)
			}
		}
	}
	if j.needClassify {
		if agg := aggregateClassifications(clsResults); agg != nil {
			agg.SHA256 = ""
			agg.CRC32 = ""
			if putErr := c.classStore.Put(agg); putErr != nil {
				c.logger.Warn("storing browse analysis classification result", "path", j.relPath, "error", putErr)
			}
		}
	}
	if j.needOCR {
		if agg := aggregateOCR(ocrResults); agg != nil {
			if putErr := c.ocrStore.Put(agg); putErr != nil {
				c.logger.Warn("storing browse analysis OCR result", "path", j.relPath, "error", putErr)
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

// aggregateOCR merges OCR results from multiple video frames, unioning distinct
// recognized blocks (first occurrence wins) so text appearing on any frame is
// captured without duplication.
func aggregateOCR(results []*ocr.Result) *ocr.Result {
	if len(results) == 0 {
		return nil
	}
	agg := *results[0]
	seen := make(map[string]struct{})
	var blocks []ocr.TextBlock
	var lines []string
	for _, r := range results {
		for _, b := range r.Blocks {
			text := strings.TrimSpace(b.Text)
			if text == "" {
				continue
			}
			if _, dup := seen[text]; dup {
				continue
			}
			seen[text] = struct{}{}
			blocks = append(blocks, b)
			lines = append(lines, text)
		}
	}
	agg.Blocks = blocks
	agg.Text = strings.Join(lines, "\n")
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
