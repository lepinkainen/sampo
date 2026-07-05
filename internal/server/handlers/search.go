package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/filesystem"
	"golang.org/x/sync/errgroup"
)

const searchResultLimit = 500

// statConcurrency bounds concurrent os.Stat calls when materializing
// store-matched paths — keeps parallelism NFS-friendly.
const statConcurrency = 16

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

	// Phase 1: Filename match via parallel directory walk. Matches are
	// collected unordered, then sorted by path and truncated. The walk is
	// cancelled once a soft limit is reached — collecting a little beyond
	// the hard limit keeps the kept 500 (lexically first) stable regardless
	// of walk order.
	const softLimit = 2 * searchResultLimit
	walkCtx, cancelWalk := context.WithCancel(r.Context())
	defer cancelWalk()

	var mu sync.Mutex
	var results []filesystem.FileEntry

	err = filesystem.WalkDirParallel(walkCtx, fullPath, func(path string, d fs.DirEntry) error {
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
		if !strings.Contains(strings.ToLower(d.Name()), queryLower) {
			return nil
		}

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
			HasThumb:  filesystem.MediaTypeHasThumb(mediaType),
		}

		mu.Lock()
		results = append(results, entry)
		full := len(results) >= softLimit
		mu.Unlock()
		if full {
			cancelWalk()
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		h.logger.Error("searching files", "error", err, "path", fullPath)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	if len(results) > searchResultLimit {
		results = results[:searchResultLimit]
	}
	seen := make(map[string]bool, len(results))
	for i := range results {
		seen[results[i].Path] = true
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

	candidates := make([]string, 0, len(paths))
	for _, relPath := range paths {
		if !seen[relPath] {
			candidates = append(candidates, relPath)
		}
	}

	// Stat candidates in parallel into an index-addressed slice so the
	// store-returned order is preserved; stale paths leave nil holes.
	entries := make([]*filesystem.FileEntry, len(candidates))
	var g errgroup.Group
	g.SetLimit(statConcurrency)
	for i, relPath := range candidates {
		g.Go(func() error {
			info, infoErr := os.Stat(filepath.Join(root.Path, relPath))
			if infoErr != nil {
				return nil
			}
			mediaType := filesystem.DetectMediaType(filepath.Base(relPath))
			entries[i] = &filesystem.FileEntry{
				Name:      filepath.Base(relPath),
				Path:      relPath,
				IsDir:     false,
				Size:      info.Size(),
				ModTime:   info.ModTime(),
				MediaType: mediaType,
				HasThumb:  filesystem.MediaTypeHasThumb(mediaType),
			}
			return nil
		})
	}
	_ = g.Wait() // goroutines never return errors

	for _, e := range entries {
		if e == nil || len(results) >= searchResultLimit {
			continue
		}
		results = append(results, *e)
		seen[e.Path] = true
	}
	return results
}
