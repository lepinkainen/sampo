package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/classification"
)

// duplicatesResponse is the JSON response for the duplicates endpoint.
type duplicatesResponse struct {
	Groups []classification.DuplicateGroup `json:"groups"`
}

// defaultSimilarityThreshold is the similarity percentage used when the
// client doesn't pass one (88% ≈ 7 Hamming bits on a 64-bit phash).
const defaultSimilarityThreshold = 88

// FindDuplicates returns groups of files with matching checksums and,
// when ?similar=true, groups of visually similar images (perceptual hash).
// ?threshold=N (80..100, default 88) sets the similarity cutoff.
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

	if r.URL.Query().Get("similar") == "true" {
		threshold := defaultSimilarityThreshold
		if t, convErr := strconv.Atoi(r.URL.Query().Get("threshold")); convErr == nil {
			threshold = min(max(t, 80), 100)
		}
		similar, simErr := h.classStore.FindSimilar(rootID, relPath, classification.SimilarityToMaxDistance(threshold))
		if simErr != nil {
			h.logger.Error("finding similar images", "error", simErr)
			http.Error(w, "Failed to find duplicates", http.StatusInternalServerError)
			return
		}
		groups = append(groups, similar...)
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
			if len(kept.Files) != len(group.Files) {
				recomputeKeeper(&kept)
			}
			filtered = append(filtered, kept)
		}
	}
	return filtered
}

// recomputeKeeper fixes the keeper index after files were pruned from a
// group. Exact groups keep any remaining copy (first); similar groups
// re-apply the resolution rule (unique best resolution wins, tie → nil).
func recomputeKeeper(group *classification.DuplicateGroup) {
	if group.Keeper == nil && group.HashType == "sha256" {
		return
	}
	keeper := 0
	if group.HashType != "phash" {
		group.Keeper = &keeper
		return
	}
	bestArea, count := -1, 0
	for i, f := range group.Files {
		area := f.Width * f.Height
		switch {
		case area > bestArea:
			bestArea, count, keeper = area, 1, i
		case area == bestArea:
			count++
		}
	}
	if count == 1 {
		group.Keeper = &keeper
	} else {
		group.Keeper = nil
	}
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
