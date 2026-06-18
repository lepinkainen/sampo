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

// OCRFile handles GET /api/ocr/{rootID}/* — runs OCR on a single image.
func (h *Handler) OCRFile(w http.ResponseWriter, r *http.Request) {
	if h.ocrStore == nil || h.ocrRecognizer == nil {
		http.Error(w, "OCR not enabled", http.StatusServiceUnavailable)
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

	// Return cached result if present.
	existing, err := h.ocrStore.Get(rootID, relPath)
	if err != nil {
		h.logger.Error("checking ocr store", "error", err)
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

	// For videos, extract a frame to OCR.
	ocrPath := fullPath
	if filesystem.DetectMediaType(fullPath) == "video" {
		framePath, cleanup, extractErr := videoframe.ExtractFrame(r.Context(), h.frameDir, fullPath)
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
		ocrPath = framePath
	}

	result, err := h.ocrRecognizer.Recognize(r.Context(), nil, ocrPath, rootID, relPath, info.ModTime().Unix(), info.Size())
	if err != nil {
		h.logger.Error("ocr failed", "error", err, "path", relPath)
		http.Error(w, "OCR failed", http.StatusInternalServerError)
		return
	}

	if err := h.ocrStore.Put(result); err != nil {
		h.logger.Error("storing ocr result", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		slog.Error("encoding ocr response", "error", err)
	}
}

type ocrScanRequest struct {
	RootID string `json:"rootId"`
	Path   string `json:"path"`
	// Force triggers a full recursive rescan that ignores cached results.
	Force bool `json:"force"`
}

// StartOCRScan handles POST /api/ocr/scan — starts a background OCR scan.
func (h *Handler) StartOCRScan(w http.ResponseWriter, r *http.Request) {
	if h.ocrScanner == nil {
		http.Error(w, "OCR not enabled", http.StatusServiceUnavailable)
		return
	}

	var req ocrScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.ocrScanner.ScanDirectory(req.RootID, req.Path, req.Force); err != nil {
		h.logger.Error("starting ocr scan", "error", err)
		http.Error(w, "Failed to start scan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.ocrScanner.Status())
}

// OCRScanStatus handles GET /api/ocr/status — returns OCR scan progress.
func (h *Handler) OCRScanStatus(w http.ResponseWriter, r *http.Request) {
	if h.ocrScanner == nil {
		http.Error(w, "OCR not enabled", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(h.ocrScanner.Status()); err != nil {
		slog.Error("encoding ocr scan status", "error", err)
	}
}
