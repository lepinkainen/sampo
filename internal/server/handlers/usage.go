package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/filesystem"
)

// DiskUsage holds directory size statistics.
type DiskUsage struct {
	TotalSize int64 `json:"totalSize"`
	FileCount int   `json:"fileCount"`
	DirCount  int   `json:"dirCount"`
}

// GetDiskUsage computes total size, file count, and directory count for a path.
func (h *Handler) GetDiskUsage(w http.ResponseWriter, r *http.Request) {
	rootID := chi.URLParam(r, "rootID")
	relPath, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if relPath == "" {
		relPath = "/"
	}

	fullPath, err := h.roots.ResolvePath(rootID, relPath)
	if err != nil {
		h.logger.Error("resolving path", "error", err, "rootID", rootID, "path", relPath)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var totalSize, fileCount, dirCount atomic.Int64
	err = filesystem.WalkDirParallel(r.Context(), fullPath, func(path string, d fs.DirEntry) error {
		// Skip hidden files/directories
		if strings.HasPrefix(d.Name(), ".") && path != fullPath {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != fullPath {
				dirCount.Add(1)
			}
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		fileCount.Add(1)
		totalSize.Add(info.Size())
		return nil
	})
	if err != nil {
		// Client disconnected mid-walk; nobody is listening for a response.
		if errors.Is(err, context.Canceled) {
			return
		}
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		h.logger.Error("computing disk usage", "error", err, "path", fullPath)
		http.Error(w, "Failed to compute disk usage", http.StatusInternalServerError)
		return
	}

	usage := DiskUsage{
		TotalSize: totalSize.Load(),
		FileCount: int(fileCount.Load()),
		DirCount:  int(dirCount.Load()),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(usage); err != nil {
		h.logger.Error("encoding usage response", "error", err)
	}
}
