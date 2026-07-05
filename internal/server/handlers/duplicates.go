package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/classification"
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
	groups = h.filterExistingDuplicateGroups(groups)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(duplicatesResponse{Groups: groups}); err != nil {
		h.logger.Error("encoding duplicates response", "error", err)
	}
}

func (h *Handler) filterExistingDuplicateGroups(groups []classification.DuplicateGroup) []classification.DuplicateGroup {
	filtered := make([]classification.DuplicateGroup, 0, len(groups))
	for _, group := range groups {
		kept := group
		kept.Files = nil
		for _, file := range group.Files {
			if h.duplicateCandidateExists(file) {
				kept.Files = append(kept.Files, file)
			}
		}
		if len(kept.Files) > 1 {
			filtered = append(filtered, kept)
		}
	}
	return filtered
}

func (h *Handler) duplicateCandidateExists(file classification.DupFile) bool {
	fullPath, err := h.roots.ResolvePath(file.RootID, file.Path)
	if err != nil {
		h.logger.Warn("resolving duplicate candidate", "error", err, "rootID", file.RootID, "path", file.Path)
		h.purgeAnalysisResults(file.RootID, file.Path)
		return false
	}
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			h.purgeAnalysisResults(file.RootID, file.Path)
			h.logger.Info("pruned missing duplicate candidate", "rootID", file.RootID, "path", file.Path)
			return false
		}
		h.logger.Warn("checking duplicate candidate", "error", err, "rootID", file.RootID, "path", file.Path)
	}
	return true
}
