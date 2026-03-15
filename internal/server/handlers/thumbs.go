package handlers

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/filemanager/internal/filesystem"
	"github.com/lepinkainen/filemanager/internal/thumbnail"
)

// GetThumbnail returns a cached thumbnail or generates one on demand.
func (h *Handler) GetThumbnail(w http.ResponseWriter, r *http.Request) {
	rootID := chi.URLParam(r, "rootID")
	relPath := chi.URLParam(r, "*")

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

	cacheKey := thumbnail.CacheKey(rootID, relPath, info.ModTime().Unix(), info.Size())

	// Check cache first
	if cachedPath, ok := h.thumbCache.Get(rootID, cacheKey); ok {
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
	mediaType := filesystem.DetectMediaType(fullPath)

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

	http.ServeFile(w, r, dstPath)
}
