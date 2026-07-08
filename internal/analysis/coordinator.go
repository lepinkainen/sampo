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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disintegration/imaging"
	"github.com/lepinkainen/sampo/internal/classification"
	"github.com/lepinkainen/sampo/internal/detection"
	"github.com/lepinkainen/sampo/internal/metadata"
	"github.com/lepinkainen/sampo/internal/ocr"
	"github.com/lepinkainen/sampo/internal/thumbnail"
	"github.com/lepinkainen/sampo/internal/videoframe"
)

// Coordinator schedules low-priority background analysis triggered by browsing.
//
// It is the single "load once, run every analyzer" path: a job opens the file
// (or extracts video frames) once and runs detection, classification, OCR, and
// optional thumbnail generation on it. Both the browse queue (Enqueue) and the
// bulk Scanner funnel through process(), so adding an analyzer here lights it
// up everywhere.
type Coordinator struct {
	detectionStore *detection.Store
	detector       *detection.Detector
	classStore     *classification.Store
	classifier     *classification.Classifier
	ocrStore       *ocr.Store
	recognizer     *ocr.Recognizer
	metaStore      *metadata.Store
	thumbCache     *thumbnail.Cache
	frameDir       string
	includeVideos  bool
	logger         *slog.Logger

	jobs chan job

	mu sync.Mutex
	// pending maps in-flight job keys to a channel closed when the job
	// finishes (or is dropped), so callers can wait without polling.
	pending map[string]chan struct{}
	active  atomic.Int32
}

type job struct {
	key       string
	rootID    string
	relPath   string
	fullPath  string
	mediaType string
	mtime     int64
	size      int64
	needs     needSet
}

// needSet marks which analyzers or shared-decode side effects must run for a file.
type needSet struct {
	detect   bool
	classify bool
	ocr      bool
	phash    bool
	thumb    bool
	meta     bool
}

func (n needSet) any() bool {
	return n.detect || n.classify || n.ocr || n.phash || n.thumb || n.meta
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
	metaStore *metadata.Store,
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
		metaStore:      metaStore,
		frameDir:       frameDir,
		includeVideos:  includeVideos,
		logger:         logger,
		jobs:           make(chan job, queueSize),
		pending:        make(map[string]chan struct{}),
	}

	for i := range workers {
		go c.worker(i)
	}

	return c
}

// SetThumbnailCache lets the coordinator create missing image thumbnails from
// the already-decoded image, avoiding a second full file read on fresh browse.
func (c *Coordinator) SetThumbnailCache(cache *thumbnail.Cache) {
	c.thumbCache = cache
}

// needs reports which enabled analyzers must run for a file. When force is true,
// every enabled analyzer runs; otherwise only those with stale cached results.
// Missing thumbnails are treated as a decode-sharing side effect for images.
func (c *Coordinator) needs(rootID, relPath, mediaType string, mtime, size int64, force bool) needSet {
	var n needSet
	if c.detectionStore != nil && c.detector != nil {
		n.detect = force || c.detectionStore.IsStale(rootID, relPath, mtime, size)
	}
	if c.classStore != nil && c.classifier != nil {
		n.classify = force || c.classStore.IsStale(rootID, relPath, mtime, size, c.classifier.ModelVersion())
	}
	if c.ocrStore != nil && c.recognizer != nil {
		n.ocr = force || c.ocrStore.IsStale(rootID, relPath, mtime, size, c.recognizer.ModelVersion())
	}
	if c.metaStore != nil {
		n.meta = force || c.metaStore.IsStale(rootID, relPath, mtime)
	}
	if c.thumbCache != nil && mediaType == "image" {
		key := thumbnail.CacheKey(rootID, relPath, mtime, size)
		_, ok := c.thumbCache.Get(rootID, key)
		n.thumb = !ok
	}
	// Perceptual hash rides along with classification (the classify Put
	// persists it); the standalone check backfills legacy NULL-phash rows
	// with a decode-only job. Videos never set phash — theirs stays NULL
	// (frames are temporary), so checking them would re-queue forever.
	if c.classStore != nil && mediaType == "image" {
		n.phash = n.classify || force || c.classStore.NeedsPHash(rootID, relPath)
	}
	return n
}

// Analyze runs all needed analyzers for a single file synchronously, loading the
// media (or extracting video frames) once. Used by the bulk Scanner.
func (c *Coordinator) Analyze(ctx context.Context, rootID, relPath, fullPath, mediaType string, mtime, size int64, force bool) {
	if c == nil {
		return
	}
	n := c.needs(rootID, relPath, mediaType, mtime, size, force)
	if !n.any() {
		return
	}
	c.process(ctx, job{
		rootID:    rootID,
		relPath:   relPath,
		fullPath:  fullPath,
		mediaType: mediaType,
		mtime:     mtime,
		size:      size,
		needs:     n,
	})
}

func (c *Coordinator) deleteCachedPath(rootID, relPath string) {
	type deletionStore interface {
		DeletePath(rootID, relPath string) error
	}
	stores := map[string]deletionStore{}
	if c.detectionStore != nil {
		stores["detection"] = c.detectionStore
	}
	if c.classStore != nil {
		stores["classification"] = c.classStore
	}
	if c.ocrStore != nil {
		stores["ocr"] = c.ocrStore
	}
	if c.metaStore != nil {
		stores["metadata"] = c.metaStore
	}
	for name, store := range stores {
		if err := store.DeletePath(rootID, relPath); err != nil {
			c.logger.Error("purging cached analysis result", "store", name, "rootID", rootID, "path", relPath, "error", err)
		}
	}
}

// PruneMissing removes cached analysis rows under relPath whose files no
// longer exist on disk, so deleted/moved files don't linger in duplicate
// finding or search. Called by the bulk Scanner at the start of a scan.
func (c *Coordinator) PruneMissing(rootID, rootPath, relPath string) {
	if c == nil {
		return
	}
	type analysisStore interface {
		ListPaths(rootID, dirPath string) ([]string, error)
		DeletePath(rootID, relPath string) error
	}
	stores := map[string]analysisStore{}
	if c.detectionStore != nil {
		stores["detection"] = c.detectionStore
	}
	if c.classStore != nil {
		stores["classification"] = c.classStore
	}
	if c.ocrStore != nil {
		stores["ocr"] = c.ocrStore
	}
	if c.metaStore != nil {
		stores["metadata"] = c.metaStore
	}

	// The stores usually cache the same files, so remember stat results to
	// avoid re-statting a path for every store.
	missing := map[string]bool{}
	for name, store := range stores {
		paths, err := store.ListPaths(rootID, relPath)
		if err != nil {
			c.logger.Error("listing cached paths for prune", "store", name, "error", err)
			continue
		}
		for _, p := range paths {
			gone, seen := missing[p]
			if !seen {
				_, statErr := os.Stat(filepath.Join(rootPath, p))
				gone = os.IsNotExist(statErr)
				missing[p] = gone
			}
			if !gone {
				continue
			}
			if err := store.DeletePath(rootID, p); err != nil {
				c.logger.Error("pruning cached result", "store", name, "path", p, "error", err)
			} else {
				c.logger.Info("pruned cached result for missing file", "store", name, "rootID", rootID, "path", p)
			}
		}
	}
}

// EnqueueItem describes a single file to analyze as part of a batch.
type EnqueueItem struct {
	RelPath   string
	FullPath  string
	MediaType string
	Mtime     int64
	Size      int64
}

// WantsMedia reports whether the coordinator analyzes the given media type:
// always images, videos only when includeVideos is set. Single source of truth
// for the browse enqueue paths and their callers.
func (c *Coordinator) WantsMedia(mediaType string) bool {
	if c == nil {
		return false
	}
	return mediaType == "image" || (c.includeVideos && mediaType == "video")
}

// claim reserves a file for analysis. It checks the pending set first so that
// concurrent callers (the per-thumbnail Enqueue and the directory EnqueueBatch)
// don't both run needs() — which hits SQLite IsStale per file — or queue a
// duplicate job. When ok is true the caller owns the pending slot and must send
// a job (or releasePending on a failed send). When ok is false nothing needs
// queuing — either the file is already in flight or all caches are fresh.
func (c *Coordinator) claim(rootID, relPath, mediaType string, mtime, size int64) (key string, n needSet, ok bool) {
	key = fmt.Sprintf("%s|%s|%d|%d", rootID, relPath, mtime, size)

	c.mu.Lock()
	_, exists := c.pending[key]
	c.mu.Unlock()
	if exists {
		return key, needSet{}, false
	}

	n = c.needs(rootID, relPath, mediaType, mtime, size, false)
	if !n.any() {
		return key, needSet{}, false
	}

	c.mu.Lock()
	if _, exists := c.pending[key]; exists {
		c.mu.Unlock()
		return key, needSet{}, false
	}
	c.pending[key] = make(chan struct{})
	c.mu.Unlock()
	return key, n, true
}

// releasePending frees a claimed slot and wakes any completion waiters.
func (c *Coordinator) releasePending(key string) {
	c.mu.Lock()
	ch := c.pending[key]
	delete(c.pending, key)
	c.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

// EnqueueBatch schedules background analysis for a batch of files (for example
// every media file in a freshly opened directory). Unlike Enqueue it never
// drops a job when the queue is full: it runs in its own goroutine and blocks
// on the send until a worker frees a slot, so directory-sized batches larger
// than the queue are fully queued rather than silently discarded. The caller
// therefore returns immediately. needs() skips files whose caches are fresh
// and the pending map dedups against in-flight jobs, so this is cheap to call
// on every directory listing.
func (c *Coordinator) EnqueueBatch(rootID string, items []EnqueueItem) {
	if c == nil {
		return
	}
	go func() {
		c.logger.Debug("browse analysis batch start", "rootID", rootID, "items", len(items))
		for _, it := range items {
			if !c.WantsMedia(it.MediaType) {
				continue
			}

			key, n, ok := c.claim(rootID, it.RelPath, it.MediaType, it.Mtime, it.Size)
			if !ok {
				continue
			}

			c.jobs <- job{
				key:       key,
				rootID:    rootID,
				relPath:   it.RelPath,
				fullPath:  it.FullPath,
				mediaType: it.MediaType,
				mtime:     it.Mtime,
				size:      it.Size,
				needs:     n,
			}
		}
	}()
}

// Enqueue schedules background analysis if at least one enabled model needs fresh data.
// Returns true if a job was queued.
func (c *Coordinator) Enqueue(rootID, relPath, fullPath, mediaType string, mtime, size int64) bool {
	if c == nil || !c.WantsMedia(mediaType) {
		return false
	}

	key, n, ok := c.claim(rootID, relPath, mediaType, mtime, size)
	if !ok {
		return false
	}

	j := job{
		key:       key,
		rootID:    rootID,
		relPath:   relPath,
		fullPath:  fullPath,
		mediaType: mediaType,
		mtime:     mtime,
		size:      size,
		needs:     n,
	}

	select {
	case c.jobs <- j:
		c.logger.Debug("browse analysis enqueued",
			"path", relPath,
			"qlen", len(c.jobs),
			"qcap", cap(c.jobs),
			"active", c.active.Load(),
		)
		return true
	default:
		c.releasePending(key)
		c.logger.Debug("browse analysis queue full; dropping job", "path", relPath)
		return false
	}
}

func (c *Coordinator) worker(id int) {
	for j := range c.jobs {
		c.active.Add(1)
		c.logger.Debug("worker picked job",
			"path", j.relPath,
			"worker", id,
			"qlen", len(c.jobs),
			"qcap", cap(c.jobs),
			"active", c.active.Load(),
		)
		start := time.Now()
		c.process(context.Background(), j)
		c.logger.Debug("worker finished job", "path", j.relPath, "worker", id, "duration_ms", time.Since(start).Milliseconds())
		c.active.Add(-1)
		c.releasePending(j.key)
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

// IsPending reports whether a job for the given file is still queued or
// processing. Callers use it to distinguish "worker hasn't finished yet"
// (keep waiting) from "worker finished without producing a result" (give up).
func (c *Coordinator) IsPending(rootID, relPath string, mtime, size int64) bool {
	if c == nil {
		return false
	}
	key := fmt.Sprintf("%s|%s|%d|%d", rootID, relPath, mtime, size)
	c.mu.Lock()
	_, exists := c.pending[key]
	c.mu.Unlock()
	return exists
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

func (c *Coordinator) generateThumbnail(ctx context.Context, j job, img image.Image) error {
	if c.thumbCache == nil {
		return nil
	}
	key := thumbnail.CacheKey(j.rootID, j.relPath, j.mtime, j.size)
	if _, ok := c.thumbCache.Get(j.rootID, key); ok {
		return nil
	}
	dstPath := c.thumbCache.Path(j.rootID, key)
	return thumbnail.GenerateImageThumbnailFromImage(ctx, img, dstPath)
}

func (c *Coordinator) processImage(ctx context.Context, j job, analyzePath string) {
	// Load once, share across every analyzer (the "same byte-level file").
	loadStart := time.Now()
	img, sha256Hex, crc32Hex, err := loadImage(analyzePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.deleteCachedPath(j.rootID, j.relPath)
			c.logger.Info("browse analysis skipped missing file; pruned cached results", "path", j.relPath)
		} else {
			c.logger.Warn("browse analysis image load failed", "path", j.relPath, "error", err)
		}
		return
	}
	loadMs := time.Since(loadStart).Milliseconds()

	// Record dimensions from the already-decoded image — cheap, and decoupled
	// from classification (which is the only other place bounds are read).
	if j.needs.meta && c.metaStore != nil {
		bounds := img.Bounds()
		if err := c.metaStore.Put(j.rootID, j.relPath, j.mtime, metadata.Dims{
			Width:  bounds.Dx(),
			Height: bounds.Dy(),
		}); err != nil {
			c.logger.Warn("storing browse analysis metadata", "path", j.relPath, "error", err)
		}
	}

	var thumbMs, detMs, clsMs, ocrMs int64

	if j.needs.thumb {
		t := time.Now()
		thumbErr := c.generateThumbnail(ctx, j, img)
		thumbMs = time.Since(t).Milliseconds()
		if thumbErr != nil {
			c.logger.Warn("browse analysis thumbnail failed", "path", j.relPath, "error", thumbErr)
		}
	}

	if j.needs.detect {
		t := time.Now()
		result, detErr := c.detector.DetectImage(img, j.rootID, j.relPath, j.mtime, j.size)
		detMs = time.Since(t).Milliseconds()
		if detErr != nil {
			c.logger.Warn("browse analysis detection failed", "path", j.relPath, "error", detErr)
		} else if putErr := c.detectionStore.Put(result); putErr != nil {
			c.logger.Warn("storing browse analysis detection result", "path", j.relPath, "error", putErr)
		}
	}

	var phash string
	var width, height int
	if j.needs.phash {
		var phErr error
		phash, phErr = classification.ComputePHash(img)
		if phErr != nil {
			c.logger.Warn("browse analysis phash failed", "path", j.relPath, "error", phErr)
		}
		bounds := img.Bounds()
		width, height = bounds.Dx(), bounds.Dy()
	}

	if j.needs.classify {
		t := time.Now()
		result, clsErr := c.classifier.ClassifyImage(img, sha256Hex, crc32Hex, j.rootID, j.relPath, j.mtime, j.size)
		clsMs = time.Since(t).Milliseconds()
		if clsErr != nil {
			c.logger.Warn("browse analysis classification failed", "path", j.relPath, "error", clsErr)
		} else {
			result.PHash = phash
			result.Width = width
			result.Height = height
			if putErr := c.classStore.Put(result); putErr != nil {
				c.logger.Warn("storing browse analysis classification result", "path", j.relPath, "error", putErr)
			}
		}
	} else if j.needs.phash && phash != "" {
		// Decode-only backfill for legacy rows: update phash and dimensions
		// without re-running any ML analyzer.
		if putErr := c.classStore.UpdatePHash(j.rootID, j.relPath, phash, width, height); putErr != nil {
			c.logger.Warn("storing browse analysis phash", "path", j.relPath, "error", putErr)
		}
	}

	if j.needs.ocr {
		// Pass the shared decode; the macOS subprocess backend ignores it and
		// re-reads analyzePath, the in-process backend reuses it.
		t := time.Now()
		result, ocrErr := c.recognizer.Recognize(ctx, img, analyzePath, j.rootID, j.relPath, j.mtime, j.size)
		ocrMs = time.Since(t).Milliseconds()
		if ocrErr != nil {
			c.logger.Warn("browse analysis OCR failed", "path", j.relPath, "error", ocrErr)
		} else if putErr := c.ocrStore.Put(result); putErr != nil {
			c.logger.Warn("storing browse analysis OCR result", "path", j.relPath, "error", putErr)
		}
	}

	c.logger.Debug("analyzed image",
		"path", j.relPath,
		"load_ms", loadMs,
		"thumb_ms", thumbMs,
		"detect_ms", detMs,
		"classify_ms", clsMs,
		"ocr_ms", ocrMs,
	)
}

func (c *Coordinator) processVideo(ctx context.Context, j job) {
	start := time.Now()

	// Record resolution/duration via a single ffprobe — cheap, and the only
	// source of video geometry since frames are decoded only for ML analysis.
	if j.needs.meta && c.metaStore != nil {
		info, probeErr := videoframe.Probe(ctx, j.fullPath)
		if probeErr != nil {
			c.logger.Warn("probing video metadata", "path", j.relPath, "error", probeErr)
		} else if err := c.metaStore.Put(j.rootID, j.relPath, j.mtime, metadata.Dims{
			Width:    info.Width,
			Height:   info.Height,
			Duration: info.Duration,
		}); err != nil {
			c.logger.Warn("storing browse analysis metadata", "path", j.relPath, "error", err)
		}
	}

	// A meta-only job (no ML analyzer needing fresh data) has nothing left to
	// do: skip the expensive frame extraction entirely.
	if !j.needs.detect && !j.needs.classify && !j.needs.ocr {
		c.logger.Debug("analyzed video metadata-only",
			"path", j.relPath,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return
	}

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

	// Decode all frames in parallel (read + JPEG decode is the parallelizable
	// part; the analyzers below serialize on ONNX session mutexes anyway).
	frameImgs := make([]image.Image, len(framePaths))
	var wg sync.WaitGroup
	for i, fp := range framePaths {
		wg.Add(1)
		go func(i int, fp string) {
			defer wg.Done()
			img, _, _, loadErr := loadImage(fp)
			if loadErr != nil {
				c.logger.Debug("browse analysis frame load failed", "path", j.relPath, "frame", fp, "error", loadErr)
				return
			}
			frameImgs[i] = img
		}(i, fp)
	}
	wg.Wait()

	// Run every analyzer on each decoded frame in order, then aggregate.
	var detResults []*detection.Result
	var clsResults []*classification.Result
	var ocrResults []*ocr.Result
	for i, fp := range framePaths {
		img := frameImgs[i]
		if img == nil {
			continue
		}

		if j.needs.detect {
			if result, detErr := c.detector.DetectImage(img, j.rootID, j.relPath, j.mtime, j.size); detErr != nil {
				c.logger.Debug("browse analysis detection failed for frame", "path", j.relPath, "frame", fp, "error", detErr)
			} else {
				detResults = append(detResults, result)
			}
		}
		if j.needs.classify {
			// Hashes left empty — frames are temporary, not the original video.
			if result, clsErr := c.classifier.ClassifyImage(img, "", "", j.rootID, j.relPath, j.mtime, j.size); clsErr != nil {
				c.logger.Debug("browse analysis classification failed for frame", "path", j.relPath, "frame", fp, "error", clsErr)
			} else {
				clsResults = append(clsResults, result)
			}
		}
		if j.needs.ocr {
			if result, ocrErr := c.recognizer.Recognize(ctx, img, fp, j.rootID, j.relPath, j.mtime, j.size); ocrErr != nil {
				c.logger.Debug("browse analysis OCR failed for frame", "path", j.relPath, "frame", fp, "error", ocrErr)
			} else {
				ocrResults = append(ocrResults, result)
			}
		}
	}

	if j.needs.detect {
		if agg := aggregateDetections(detResults); agg != nil {
			if putErr := c.detectionStore.Put(agg); putErr != nil {
				c.logger.Warn("storing browse analysis detection result", "path", j.relPath, "error", putErr)
			}
		}
	}
	if j.needs.classify {
		if agg := aggregateClassifications(clsResults); agg != nil {
			agg.SHA256 = ""
			agg.CRC32 = ""
			if putErr := c.classStore.Put(agg); putErr != nil {
				c.logger.Warn("storing browse analysis classification result", "path", j.relPath, "error", putErr)
			}
		}
	}
	if j.needs.ocr {
		if agg := aggregateOCR(ocrResults); agg != nil {
			if putErr := c.ocrStore.Put(agg); putErr != nil {
				c.logger.Warn("storing browse analysis OCR result", "path", j.relPath, "error", putErr)
			}
		}
	}

	c.logger.Debug("analyzed video",
		"path", j.relPath,
		"frames", len(framePaths),
		"duration_ms", time.Since(start).Milliseconds(),
	)
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
