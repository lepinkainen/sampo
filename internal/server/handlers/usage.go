package handlers

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
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

	var usage DiskUsage
	err = filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't read
		}
		// Skip hidden files/directories
		if strings.HasPrefix(d.Name(), ".") && path != fullPath {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != fullPath {
				usage.DirCount++
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		usage.FileCount++
		usage.TotalSize += info.Size()
		return nil
	})
	if err != nil {
		h.logger.Error("computing disk usage", "error", err, "path", fullPath)
		http.Error(w, "Failed to compute disk usage", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(usage); err != nil {
		h.logger.Error("encoding usage response", "error", err)
	}
}
