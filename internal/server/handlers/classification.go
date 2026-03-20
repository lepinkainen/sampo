package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/filemanager/internal/filesystem"
	"github.com/lepinkainen/filemanager/internal/videoframe"
)

// ClassifyFile handles GET /api/classify/{rootID}/* — runs classification on a single image.
func (h *Handler) ClassifyFile(w http.ResponseWriter, r *http.Request) {
	if h.classStore == nil || h.classifier == nil {
		http.Error(w, "Classification not enabled", http.StatusServiceUnavailable)
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
	existing, err := h.classStore.Get(rootID, relPath)
	if err != nil {
		h.logger.Error("checking classification store", "error", err)
	}
	if existing != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(existing)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// For video files, extract a frame to analyze
	classifyPath := fullPath
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
		classifyPath = framePath
	}

	result, err := h.classifier.Classify(classifyPath, rootID, relPath, info.ModTime().Unix(), info.Size())
	if err != nil {
		h.logger.Error("classification failed", "error", err, "path", relPath)
		http.Error(w, "Classification failed", http.StatusInternalServerError)
		return
	}

	if err := h.classStore.Put(result); err != nil {
		h.logger.Error("storing classification result", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("encoding classification response", "error", err)
	}
}

type classifyScanRequest struct {
	RootID string `json:"rootId"`
	Path   string `json:"path"`
}

// StartClassifyScan handles POST /api/classify/scan — starts a background classification scan.
func (h *Handler) StartClassifyScan(w http.ResponseWriter, r *http.Request) {
	if h.classScanner == nil {
		http.Error(w, "Classification not enabled", http.StatusServiceUnavailable)
		return
	}

	var req classifyScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.classScanner.ScanDirectory(req.RootID, req.Path); err != nil {
		h.logger.Error("starting classification scan", "error", err)
		http.Error(w, "Failed to start scan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.classScanner.Status())
}

// ClassifyScanStatus handles GET /api/classify/status — returns classification scan progress.
func (h *Handler) ClassifyScanStatus(w http.ResponseWriter, r *http.Request) {
	if h.classScanner == nil {
		http.Error(w, "Classification not enabled", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(h.classScanner.Status()); err != nil {
		slog.Error("encoding classification scan status", "error", err)
	}
}
