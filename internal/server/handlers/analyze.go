package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type analyzeScanRequest struct {
	RootID string `json:"rootId"`
	Path   string `json:"path"`
	// Force re-runs every analyzer on every file, ignoring cached results.
	Force bool `json:"force"`
}

// StartAnalyzeScan handles POST /api/analyze/scan — starts a unified scan that
// loads each file once and runs every enabled analyzer (detection, tags, OCR).
func (h *Handler) StartAnalyzeScan(w http.ResponseWriter, r *http.Request) {
	if h.analysisScanner == nil {
		http.Error(w, "Analysis not enabled", http.StatusServiceUnavailable)
		return
	}

	var req analyzeScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.analysisScanner.ScanDirectory(req.RootID, req.Path, req.Force); err != nil {
		h.logger.Error("starting analysis scan", "error", err)
		http.Error(w, "Failed to start scan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.analysisScanner.Status())
}

// AnalyzeScanStatus handles GET /api/analyze/status — returns unified scan progress.
func (h *Handler) AnalyzeScanStatus(w http.ResponseWriter, r *http.Request) {
	if h.analysisScanner == nil {
		http.Error(w, "Analysis not enabled", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(h.analysisScanner.Status()); err != nil {
		slog.Error("encoding analysis scan status", "error", err)
	}
}
