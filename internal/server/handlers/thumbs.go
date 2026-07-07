package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/analysis"
	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/thumbnail"
)

const autoBrowseThumbnailWait = 2 * time.Second

// GetThumbnail returns a cached thumbnail or generates one on demand.
func (h *Handler) GetThumbnail(w http.ResponseWriter, r *http.Request) {
	rootID := chi.URLParam(r, "rootID")
	relPath, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	fullPath, err := h.roots.ResolvePath(rootID, relPath)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		h.serveDirThumbnail(w, r, rootID, relPath, fullPath)
		return
	}

	mtime := info.ModTime().Unix()
	size := info.Size()
	cacheKey := thumbnail.CacheKey(rootID, relPath, mtime, size)
	mediaType := filesystem.DetectMediaType(fullPath)

	// Check cache first.
	if cachedPath, ok := h.thumbCache.Get(rootID, cacheKey); ok {
		h.enqueueBrowseAnalysis(rootID, relPath, fullPath, mediaType, mtime, size)
		http.ServeFile(w, r, cachedPath)
		return
	}

	// When auto-browse analysis is on, image thumbnails are generated inside the
	// load-once analysis worker. Avoid generating them here too, which would read
	// and decode the same fresh file a second time.
	if mediaType == "image" && h.deferImageThumbnailToBrowseAnalysis(w, r, rootID, relPath, fullPath, cacheKey, mtime, size) {
		return
	}

	// Generate thumbnail directly for non-ML media, or when auto-browse analysis
	// is off/unavailable.
	ensureErr := h.thumbCache.EnsureDir(rootID)
	if ensureErr != nil {
		h.logger.Error("creating cache dir", "error", ensureErr)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	dstPath := h.thumbCache.Path(rootID, cacheKey)

	switch mediaType {
	case "image":
		err = thumbnail.GenerateImageThumbnail(r.Context(), fullPath, dstPath)
	case "video":
		err = thumbnail.GenerateVideoThumbnail(r.Context(), fullPath, dstPath)
	case "pdf":
		err = thumbnail.GeneratePdfThumbnail(r.Context(), fullPath, dstPath)
	default:
		http.Error(w, "No thumbnail available", http.StatusNotFound)
		return
	}

	if err != nil {
		h.logger.Error("generating thumbnail", "error", err, "path", fullPath)
		http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
		return
	}

	h.enqueueBrowseAnalysis(rootID, relPath, fullPath, mediaType, mtime, size)
	http.ServeFile(w, r, dstPath)
}

func (h *Handler) deferImageThumbnailToBrowseAnalysis(w http.ResponseWriter, r *http.Request, rootID, relPath, fullPath, cacheKey string, mtime, size int64) bool {
	if !h.AutoBrowseEnabled() || h.browseCoordinator == nil {
		return false
	}

	// EnqueueDone never drops the job on a full queue and returns a channel
	// closed when the worker finishes, so no cache polling is needed. A nil
	// channel means nothing needs running — the thumbnail may have appeared
	// since the caller's cache check, so fall through to the re-check below.
	done := h.browseCoordinator.EnqueueDone(rootID, analysis.EnqueueItem{
		RelPath:   relPath,
		FullPath:  fullPath,
		MediaType: "image",
		Mtime:     mtime,
		Size:      size,
	})
	if done != nil {
		deadline := time.NewTimer(autoBrowseThumbnailWait)
		defer deadline.Stop()
		select {
		case <-r.Context().Done():
			return false
		case <-deadline.C:
			// Queue backed up — fall through to synchronous generation,
			// which gives a definitive 200 or 500. No 202 retry loop.
		case <-done:
			// Worker finished. A cache miss below means the image is
			// undecodable; sync generation returns the definitive error.
		}
	}

	if cachedPath, ok := h.thumbCache.Get(rootID, cacheKey); ok {
		http.ServeFile(w, r, cachedPath)
		return true
	}
	return false
}

func (h *Handler) serveDirThumbnail(w http.ResponseWriter, r *http.Request, rootID, relPath, fullPath string) {
	images, err := filesystem.ImageFilesInDir(fullPath, relPath)
	if err != nil {
		h.logger.Error("reading directory for thumbnail", "error", err, "path", fullPath)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	thumbPath, err := thumbnail.FindFirstCachedOrGenerate(r.Context(), images, rootID, h.thumbCache)
	if err != nil {
		if errors.Is(err, thumbnail.ErrNoImages) {
			http.Error(w, "No thumbnail available", http.StatusNotFound)
			return
		}
		h.logger.Error("generating directory thumbnail", "error", err, "path", fullPath)
		http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, thumbPath)
}

func (h *Handler) enqueueBrowseAnalysis(rootID, relPath, fullPath, mediaType string, mtime, size int64) {
	if !h.AutoBrowseEnabled() || h.browseCoordinator == nil {
		return
	}
	_ = h.browseCoordinator.Enqueue(rootID, relPath, fullPath, mediaType, mtime, size)
}
