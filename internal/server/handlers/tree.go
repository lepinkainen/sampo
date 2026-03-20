package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/filemanager/internal/filesystem"
)

// ListDirectory returns the contents of a directory within a root.
func (h *Handler) ListDirectory(w http.ResponseWriter, r *http.Request) {
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

	entries, err := filesystem.ListDirectory(fullPath, relPath)
	if err != nil {
		h.logger.Error("listing directory", "error", err, "path", fullPath)
		http.Error(w, "Failed to list directory", http.StatusInternalServerError)
		return
	}

	// Enrich entries with detection data and optionally filter
	if h.detectionStore != nil {
		detections, err := h.detectionStore.GetDirDetections(rootID, relPath)
		if err != nil {
			h.logger.Error("getting dir detections", "error", err)
		} else if len(detections) > 0 {
			filter := r.URL.Query().Get("filter")
			filtered := make([]filesystem.FileEntry, 0, len(entries))
			for i := range entries {
				if hasPerson, ok := detections[entries[i].Path]; ok {
					if filter == "no-people" && hasPerson {
						continue
					}
					v := hasPerson
					entries[i].HasPerson = &v
				}
				filtered = append(filtered, entries[i])
			}
			entries = filtered
		}
	}

	// Enrich entries with classification tags and optionally filter by tag
	if h.classStore != nil {
		tagFilter := r.URL.Query().Get("tag")

		// If filtering by tag, get the matching set first
		var tagMatches map[string]bool
		if tagFilter != "" {
			var err error
			tagMatches, err = h.classStore.FilterByTag(rootID, relPath, tagFilter, 0)
			if err != nil {
				h.logger.Error("filtering by tag", "error", err)
			}
		}

		dirTags, err := h.classStore.GetDirTags(rootID, relPath)
		if err != nil {
			h.logger.Error("getting dir tags", "error", err)
		} else if len(dirTags) > 0 || tagMatches != nil {
			filtered := make([]filesystem.FileEntry, 0, len(entries))
			for i := range entries {
				// Apply tag filter
				if tagMatches != nil && !entries[i].IsDir && !tagMatches[entries[i].Path] {
					continue
				}
				// Enrich with tags
				if tags, ok := dirTags[entries[i].Path]; ok {
					entries[i].Tags = make([]filesystem.TagScore, len(tags))
					for j, t := range tags {
						entries[i].Tags[j] = filesystem.TagScore{Label: t.Label, Score: t.Score}
					}
				}
				filtered = append(filtered, entries[i])
			}
			entries = filtered
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		slog.Error("encoding directory response", "error", err)
	}
}
