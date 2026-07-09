package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lepinkainen/sampo/internal/stash"
)

// organizeFile is a file entry returned in the organize suggestion response.
type organizeFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// organizeGroup is a set of files that should move to the same performer directory.
type organizeGroup struct {
	Performer  string         `json:"performer"`
	TargetRoot string         `json:"targetRoot"`
	TargetPath string         `json:"targetPath"`
	Exists     bool           `json:"exists"`
	MatchedBy  string         `json:"matchedBy"`
	Candidates []string       `json:"candidates"`
	Files      []organizeFile `json:"files"`
}

// organizeResponse is the response body for POST /api/organize/suggest.
type organizeResponse struct {
	Groups    []organizeGroup `json:"groups"`
	Unmatched []organizeFile  `json:"unmatched"`
}

// organizeRequest is the request body for POST /api/organize/suggest.
type organizeRequest struct {
	RootID string `json:"rootId"`
	Path   string `json:"path"`
}

// organizeStatusResponse is the response body for GET /api/organize/status.
type organizeStatusResponse struct {
	Enabled       bool   `json:"enabled"`
	ArchiveRootID string `json:"archiveRootId"`
}

// organizePerformersResponse is the response body for GET /api/organize/performers.
type organizePerformersResponse struct {
	Performers []string `json:"performers"`
}

// OrganizeStatus handles GET /api/organize/status.
// It returns {"enabled": true/false, "archiveRootId": "..."} so the frontend
// can show/hide the button without the API key ever being sent to the browser.
func (h *Handler) OrganizeStatus(w http.ResponseWriter, r *http.Request) {
	archiveRootID := ""
	if h.stashClient != nil {
		archiveRootID = h.archiveRootID
	}
	resp := organizeStatusResponse{
		Enabled:       h.stashClient != nil,
		ArchiveRootID: archiveRootID,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("encoding organize status response", "error", err)
	}
}

// OrganizePerformers handles GET /api/organize/performers.
// It returns a sorted list of performer names from Stash.
func (h *Handler) OrganizePerformers(w http.ResponseWriter, r *http.Request) {
	if h.stashClient == nil {
		http.Error(w, "Stash integration not configured", http.StatusServiceUnavailable)
		return
	}

	performers, err := h.stashClient.Performers(r.Context())
	if err != nil {
		h.logger.Error("fetching performers from Stash", "error", err)
		http.Error(w, "Failed to fetch performers from Stash", http.StatusBadGateway)
		return
	}

	names := make([]string, 0, len(performers))
	for _, p := range performers {
		names = append(names, p.Name)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})

	resp := organizePerformersResponse{Performers: names}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("encoding organize performers response", "error", err)
	}
}

// SuggestOrganize handles POST /api/organize/suggest.
// It fetches performers from StashApp, lists existing archive dirs, matches
// the source directory's files against ALL performers, then preselects the
// target dir: existing dirs use their exact-case name (exists=true), new
// dirs use the performer name (exists=false).
func (h *Handler) SuggestOrganize(w http.ResponseWriter, r *http.Request) {
	if h.stashClient == nil {
		http.Error(w, "Stash integration not configured", http.StatusServiceUnavailable)
		return
	}
	if h.archiveRootID == "" {
		http.Error(w, "Archive root not configured", http.StatusServiceUnavailable)
		return
	}

	var req organizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.RootID == "" || req.Path == "" {
		http.Error(w, "rootId and path are required", http.StatusBadRequest)
		return
	}

	// Fetch performers from Stash (cached).
	performers, err := h.stashClient.Performers(r.Context())
	if err != nil {
		h.logger.Error("fetching performers from Stash", "error", err)
		http.Error(w, "Failed to fetch performers from Stash", http.StatusBadGateway)
		return
	}

	// Resolve the archive root and list its top-level subdirectories.
	archivePath, err := h.roots.ResolvePath(h.archiveRootID, "/")
	if err != nil {
		h.logger.Error("resolving archive root", "error", err, "rootID", h.archiveRootID)
		http.Error(w, "Archive root unavailable", http.StatusInternalServerError)
		return
	}

	archiveEntries, err := os.ReadDir(archivePath)
	if err != nil {
		h.logger.Error("reading archive root", "error", err, "path", archivePath)
		http.Error(w, "Failed to read archive root", http.StatusInternalServerError)
		return
	}

	// Build candidatesByLeaf: lowercase(leafName) → []relPath (sorted).
	// Scan depth 1 (top-level dirs) and depth 2 (their immediate subdirs).
	candidatesByLeaf := make(map[string][]string)
	for _, d1 := range archiveEntries {
		if !d1.IsDir() || strings.HasPrefix(d1.Name(), ".") {
			continue
		}
		// Depth 1: the top-level dir itself is a candidate leaf.
		key1 := strings.ToLower(d1.Name())
		candidatesByLeaf[key1] = append(candidatesByLeaf[key1], d1.Name())

		// Depth 2: subdirs inside d1.
		d2Entries, d2Err := os.ReadDir(filepath.Join(archivePath, d1.Name()))
		if d2Err != nil {
			h.logger.Debug("reading archive subdir", "dir", d1.Name(), "error", d2Err)
			continue
		}
		for _, d2 := range d2Entries {
			if !d2.IsDir() || strings.HasPrefix(d2.Name(), ".") {
				continue
			}
			key2 := strings.ToLower(d2.Name())
			relPath := d1.Name() + "/" + d2.Name()
			candidatesByLeaf[key2] = append(candidatesByLeaf[key2], relPath)
		}
	}
	// Sort each candidate slice for deterministic output.
	for k := range candidatesByLeaf {
		sort.Strings(candidatesByLeaf[k])
	}

	// Resolve source directory and list its files (non-recursive, non-dotfiles).
	srcPath, err := h.roots.ResolvePath(req.RootID, req.Path)
	if err != nil {
		h.logger.Error("resolving source path", "error", err, "rootID", req.RootID, "path", req.Path)
		http.Error(w, "Source path not found", http.StatusNotFound)
		return
	}

	srcEntries, err := os.ReadDir(srcPath)
	if err != nil {
		h.logger.Error("reading source directory", "error", err, "path", srcPath)
		http.Error(w, "Failed to read source directory", http.StatusInternalServerError)
		return
	}

	// Collect files (skip subdirs and dotfiles).
	var files [][2]string
	for _, e := range srcEntries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		// Path relative to the source root is the requested path + filename.
		relPath := strings.TrimPrefix(req.Path+"/"+e.Name(), "/")
		files = append(files, [2]string{relPath, e.Name()})
	}

	// Determine the directory name for dirname-fallback matching.
	srcDirName := req.Path
	if idx := strings.LastIndex(srcDirName, "/"); idx >= 0 {
		srcDirName = srcDirName[idx+1:]
	}

	// Run the matcher against ALL performers (no existing-dir filter).
	matchGroups, unmatchedFiles := stash.Match(performers, srcDirName, files)

	// Build response groups: preselect target dir based on archive existence.
	respGroups := make([]organizeGroup, 0, len(matchGroups))
	for _, mg := range matchGroups {
		of := make([]organizeFile, len(mg.Files))
		for i, f := range mg.Files {
			of[i] = organizeFile{Path: f.Path, Name: f.Name}
		}

		// Determine target path, existence, and candidates.
		candidates, found := candidatesByLeaf[strings.ToLower(mg.Performer)]
		var targetPath string
		var exists bool
		if !found || len(candidates) == 0 {
			// No match: propose a new dir named after the performer.
			targetPath = mg.Performer
			exists = false
			candidates = []string{}
		} else {
			targetPath = candidates[0]
			exists = true
		}

		respGroups = append(respGroups, organizeGroup{
			Performer:  mg.Performer,
			TargetRoot: h.archiveRootID,
			TargetPath: targetPath,
			Exists:     exists,
			MatchedBy:  string(mg.MatchedBy),
			Candidates: candidates,
			Files:      of,
		})
	}

	// Build unmatched list.
	respUnmatched := make([]organizeFile, len(unmatchedFiles))
	for i, f := range unmatchedFiles {
		respUnmatched[i] = organizeFile{Path: f.Path, Name: f.Name}
	}

	resp := organizeResponse{
		Groups:    respGroups,
		Unmatched: respUnmatched,
	}

	h.logger.Info("organize suggest",
		"srcRoot", req.RootID,
		"srcPath", req.Path,
		"groups", len(respGroups),
		"unmatched", len(respUnmatched),
	)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("encoding organize response", "error", err)
	}
}
