package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/thumbnail"
)

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

	cacheKey := thumbnail.CacheKey(rootID, relPath, info.ModTime().Unix(), info.Size())
	mediaType := filesystem.DetectMediaType(fullPath)

	// Check cache first
	if cachedPath, ok := h.thumbCache.Get(rootID, cacheKey); ok {
		h.enqueueBrowseAnalysis(rootID, relPath, fullPath, mediaType, info.ModTime().Unix(), info.Size())
		http.ServeFile(w, r, cachedPath)
		return
	}

	// Generate thumbnail
	ensureErr := h.thumbCache.EnsureDir(rootID)
	if ensureErr != nil {
		h.logger.Error("creating cache dir", "error", ensureErr)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	dstPath := h.thumbCache.Path(rootID, cacheKey)

	switch mediaType {
	case "image":
		err = thumbnail.GenerateImageThumbnail(fullPath, dstPath)
	case "video":
		err = thumbnail.GenerateVideoThumbnail(fullPath, dstPath)
	default:
		http.Error(w, "No thumbnail available", http.StatusNotFound)
		return
	}

	if err != nil {
		h.logger.Error("generating thumbnail", "error", err, "path", fullPath)
		http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
		return
	}

	h.enqueueBrowseAnalysis(rootID, relPath, fullPath, mediaType, info.ModTime().Unix(), info.Size())
	http.ServeFile(w, r, dstPath)
}

func (h *Handler) serveDirThumbnail(w http.ResponseWriter, r *http.Request, rootID, relPath, fullPath string) {
	images, err := filesystem.ImageFilesInDir(fullPath, relPath)
	if err != nil {
		h.logger.Error("reading directory for thumbnail", "error", err, "path", fullPath)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	thumbPath, err := thumbnail.FindFirstCachedOrGenerate(images, rootID, h.thumbCache)
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
