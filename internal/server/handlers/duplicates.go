package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/filemanager/internal/classification"
)

// duplicatesResponse is the JSON response for the duplicates endpoint.
type duplicatesResponse struct {
	Groups []classification.DuplicateGroup `json:"groups"`
}

// FindDuplicates returns groups of files with matching checksums.
func (h *Handler) FindDuplicates(w http.ResponseWriter, r *http.Request) {
	if h.classStore == nil {
		http.Error(w, "Classification not enabled", http.StatusServiceUnavailable)
		return
	}

	rootID := chi.URLParam(r, "rootID")
	relPath, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if relPath == "" {
		relPath = "/"
	}

	// Verify the path exists
	_, err = h.roots.ResolvePath(rootID, relPath)
	if err != nil {
		h.logger.Error("resolving path", "error", err, "rootID", rootID, "path", relPath)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	groups, err := h.classStore.FindDuplicates(rootID, relPath)
	if err != nil {
		h.logger.Error("finding duplicates", "error", err)
		http.Error(w, "Failed to find duplicates", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(duplicatesResponse{Groups: groups}); err != nil {
		h.logger.Error("encoding duplicates response", "error", err)
	}
}
