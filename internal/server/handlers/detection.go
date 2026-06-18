package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/videoframe"
)

// DetectFile handles GET /api/detect/{rootID}/* — runs detection on a single image.
func (h *Handler) DetectFile(w http.ResponseWriter, r *http.Request) {
	if h.detectionStore == nil || h.detector == nil {
		http.Error(w, "Detection not enabled", http.StatusServiceUnavailable)
		return
	}

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

	// Check if we already have a fresh result
	existing, err := h.detectionStore.Get(rootID, relPath)
	if err != nil {
		h.logger.Error("checking detection store", "error", err)
	}
	if existing != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(existing)
		return
	}

	// Get file info for mtime/size
	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// For video files, extract a frame to analyze
	detectPath := fullPath
	if filesystem.DetectMediaType(fullPath) == "video" {
		framePath, cleanup, extractErr := videoframe.ExtractFrame(h.frameDir, fullPath)
		if extractErr != nil {
			h.logger.Error("video frame extraction failed", "error", extractErr, "path", relPath)
			http.Error(w, "Video frame extraction failed", http.StatusInternalServerError)
			return
		}
		defer func() {
			if cleanupErr := cleanup(); cleanupErr != nil {
				h.logger.Warn("failed to remove temp video frame", "path", framePath, "error", cleanupErr)
			}
		}()
		detectPath = framePath
	}

	result, err := h.detector.Detect(detectPath, rootID, relPath, info.ModTime().Unix(), info.Size())
	if err != nil {
		h.logger.Error("detection failed", "error", err, "path", relPath)
		http.Error(w, "Detection failed", http.StatusInternalServerError)
		return
	}

	if err := h.detectionStore.Put(result); err != nil {
		h.logger.Error("storing detection result", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("encoding detection response", "error", err)
	}
}

type scanRequest struct {
	RootID string `json:"rootId"`
	Path   string `json:"path"`
}

// StartScan handles POST /api/detect/scan — starts a background scan.
func (h *Handler) StartScan(w http.ResponseWriter, r *http.Request) {
	if h.scanner == nil {
		http.Error(w, "Detection not enabled", http.StatusServiceUnavailable)
		return
	}

	var req scanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.scanner.ScanDirectory(req.RootID, req.Path); err != nil {
		h.logger.Error("starting scan", "error", err)
		http.Error(w, "Failed to start scan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.scanner.Status())
}

// ScanStatus handles GET /api/detect/status — returns scan progress.
func (h *Handler) ScanStatus(w http.ResponseWriter, r *http.Request) {
	if h.scanner == nil {
		http.Error(w, "Detection not enabled", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(h.scanner.Status()); err != nil {
		slog.Error("encoding scan status", "error", err)
	}
}
