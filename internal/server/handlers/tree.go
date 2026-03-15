package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/filemanager/internal/filesystem"
)

// ListDirectory returns the contents of a directory within a root.
func (h *Handler) ListDirectory(w http.ResponseWriter, r *http.Request) {
	rootID := chi.URLParam(r, "rootID")
	relPath := chi.URLParam(r, "*")

	if relPath == "" {
		relPath = "/"
	}

	fullPath, err := h.roots.ResolvePath(rootID, relPath)
	if err != nil {
		h.logger.Error("resolving path", "error", err, "rootID", rootID, "path", relPath)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	entries, err := filesystem.ListDirectory(fullPath, relPath)
	if err != nil {
		h.logger.Error("listing directory", "error", err, "path", fullPath)
		http.Error(w, "Failed to list directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		slog.Error("encoding directory response", "error", err)
	}
}
