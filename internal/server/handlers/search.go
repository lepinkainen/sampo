package handlers

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/filesystem"
)

const searchResultLimit = 500

// SearchFiles searches for files by name and optionally by classification tags.
func (h *Handler) SearchFiles(w http.ResponseWriter, r *http.Request) {
	rootID := chi.URLParam(r, "rootID")
	query := r.URL.Query().Get("q")
	scopePath := r.URL.Query().Get("path")

	if query == "" {
		http.Error(w, "Missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	if scopePath == "" {
		scopePath = "/"
	}

	fullPath, err := h.roots.ResolvePath(rootID, scopePath)
	if err != nil {
		h.logger.Error("resolving path", "error", err, "rootID", rootID, "path", scopePath)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	queryLower := strings.ToLower(query)

	// Phase 1: Filename match via directory walk
	seen := make(map[string]bool)
	var results []filesystem.FileEntry

	err = filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden files/directories
		if strings.HasPrefix(d.Name(), ".") && path != fullPath {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if len(results) >= searchResultLimit {
			return filepath.SkipAll
		}

		if strings.Contains(strings.ToLower(d.Name()), queryLower) {
			info, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}

			relPath, relErr := filepath.Rel(fullPath, path)
			if relErr != nil {
				return nil
			}
			// Construct relPath relative to root (include scopePath)
			entryRelPath := filepath.Join(scopePath, relPath)

			mediaType := filesystem.DetectMediaType(d.Name())
			entry := filesystem.FileEntry{
				Name:      d.Name(),
				Path:      entryRelPath,
				IsDir:     false,
				Size:      info.Size(),
				ModTime:   info.ModTime(),
				MediaType: mediaType,
				HasThumb:  mediaType == "image" || mediaType == "video",
			}
			results = append(results, entry)
			seen[entryRelPath] = true
		}
		return nil
	})
	if err != nil {
		h.logger.Error("searching files", "error", err, "path", fullPath)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	// Phase 2: Tag match (if classification store is available)
	if h.classStore != nil && len(results) < searchResultLimit {
		tagPaths, err := h.classStore.SearchByTag(rootID, scopePath, queryLower)
		if err != nil {
			h.logger.Error("searching by tag", "error", err)
		} else {
			results = h.appendPathMatches(rootID, tagPaths, results, seen)
		}
	}

	// Phase 3: OCR text match (if OCR store is available)
	if h.ocrStore != nil && len(results) < searchResultLimit {
		ocrPaths, err := h.ocrStore.SearchByText(rootID, scopePath, queryLower)
		if err != nil {
			h.logger.Error("searching by ocr text", "error", err)
		} else {
			results = h.appendPathMatches(rootID, ocrPaths, results, seen)
		}
	}

	// Enrich results with tags from classification store
	if h.classStore != nil {
		for i := range results {
			tags, err := h.classStore.GetFileTags(rootID, results[i].Path)
			if err != nil {
				continue
			}
			if len(tags) > 0 {
				results[i].Tags = make([]filesystem.TagScore, len(tags))
				for j, t := range tags {
					results[i].Tags[j] = filesystem.TagScore{Label: t.Label, Score: t.Score}
				}
			}
		}
	}

	// Enrich results with recognized text from OCR store
	if h.ocrStore != nil {
		for i := range results {
			text, err := h.ocrStore.GetText(rootID, results[i].Path)
			if err != nil {
				continue
			}
			results[i].OCRText = text
		}
	}

	// Enrich with detection data
	if h.detectionStore != nil {
		for i := range results {
			hasPerson, err := h.detectionStore.GetDetection(rootID, results[i].Path)
			if err == nil {
				results[i].HasPerson = &hasPerson
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		h.logger.Error("encoding search response", "error", err)
	}
}

// appendPathMatches appends file entries for store-matched rel paths (from tag
// or OCR-text search), skipping already-seen paths and respecting the result
// limit. Paths that no longer exist on disk are silently dropped.
func (h *Handler) appendPathMatches(rootID string, paths []string, results []filesystem.FileEntry, seen map[string]bool) []filesystem.FileEntry {
	root, err := h.roots.Get(rootID)
	if err != nil {
		return results
	}
	for _, relPath := range paths {
		if seen[relPath] || len(results) >= searchResultLimit {
			continue
		}
		entryFullPath := filepath.Join(root.Path, relPath)
		info, infoErr := os.Stat(entryFullPath)
		if infoErr != nil {
			continue
		}
		mediaType := filesystem.DetectMediaType(filepath.Base(relPath))
		results = append(results, filesystem.FileEntry{
			Name:      filepath.Base(relPath),
			Path:      relPath,
			IsDir:     false,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			MediaType: mediaType,
			HasThumb:  mediaType == "image" || mediaType == "video",
		})
		seen[relPath] = true
	}
	return results
}
