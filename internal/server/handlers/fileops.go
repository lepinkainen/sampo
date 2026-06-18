package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/filesystem"
)

// DeleteFile handles DELETE /api/files/{rootID}/*
func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	rootID := chi.URLParam(r, "rootID")
	relPath, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if relPath == "" || relPath == "/" {
		http.Error(w, "Cannot delete root", http.StatusBadRequest)
		return
	}

	fullPath, err := h.roots.ResolvePath(rootID, relPath)
	if err != nil {
		h.logger.Error("resolving path", "error", err, "rootID", rootID, "path", relPath)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	recursive := r.URL.Query().Get("recursive") == "true"

	if err := filesystem.Delete(fullPath, recursive); err != nil {
		h.logger.Error("deleting file", "error", err, "path", fullPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info("deleted", "rootID", rootID, "path", relPath, "recursive", recursive)
	w.WriteHeader(http.StatusNoContent)
}

type fileItem struct {
	SrcRoot string `json:"srcRoot"`
	SrcPath string `json:"srcPath"`
}

type bulkRequest struct {
	Items   []fileItem `json:"items"`
	DstRoot string     `json:"dstRoot"`
	DstPath string     `json:"dstPath"`
}

type itemResult struct {
	SrcRoot string `json:"srcRoot"`
	SrcPath string `json:"srcPath"`
	DstPath string `json:"dstPath,omitempty"`
	Error   string `json:"error,omitempty"`
}

// MoveFiles handles POST /api/files/move
func (h *Handler) MoveFiles(w http.ResponseWriter, r *http.Request) {
	h.bulkOp(w, r, "move")
}

// CopyFiles handles POST /api/files/copy
func (h *Handler) CopyFiles(w http.ResponseWriter, r *http.Request) {
	h.bulkOp(w, r, "copy")
}

func (h *Handler) bulkOp(w http.ResponseWriter, r *http.Request, op string) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Items) == 0 {
		http.Error(w, "No items specified", http.StatusBadRequest)
		return
	}

	// Resolve destination directory
	dstDir, err := h.roots.ResolvePath(req.DstRoot, req.DstPath)
	if err != nil {
		h.logger.Error("resolving destination", "error", err, "rootID", req.DstRoot, "path", req.DstPath)
		http.Error(w, "Invalid destination", http.StatusBadRequest)
		return
	}

	results := make([]itemResult, 0, len(req.Items))
	hasError := false

	for _, item := range req.Items {
		srcFull, err := h.roots.ResolvePath(item.SrcRoot, item.SrcPath)
		if err != nil {
			results = append(results, itemResult{
				SrcRoot: item.SrcRoot,
				SrcPath: item.SrcPath,
				Error:   "source not found",
			})
			hasError = true
			continue
		}

		dstFull := filepath.Join(dstDir, filepath.Base(srcFull))

		var actualDst string
		if op == "move" {
			actualDst, err = filesystem.MoveFile(srcFull, dstFull)
		} else {
			info, statErr := filesystem.StatPath(srcFull)
			if statErr != nil {
				err = statErr
			} else if info.IsDir() {
				err = filesystem.CopyDir(srcFull, dstFull)
				actualDst = dstFull
			} else {
				actualDst, err = filesystem.CopyFile(srcFull, dstFull)
			}
		}

		res := itemResult{
			SrcRoot: item.SrcRoot,
			SrcPath: item.SrcPath,
		}
		if err != nil {
			res.Error = err.Error()
			hasError = true
			h.logger.Error(op+" failed", "error", err, "src", srcFull, "dst", dstFull)
		} else {
			// Return relative destination path
			res.DstPath = filepath.Base(actualDst)
			h.logger.Info(op+" completed", "src", item.SrcPath, "dst", actualDst)
		}
		results = append(results, res)
	}

	status := http.StatusOK
	if hasError {
		if allErrors(results) {
			status = http.StatusInternalServerError
		} else {
			status = http.StatusMultiStatus
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(results); err != nil {
		slog.Error("encoding response", "error", err)
	}
}

type renameRequest struct {
	RootID  string `json:"rootId"`
	Path    string `json:"path"`
	NewName string `json:"newName"`
}

// RenameFile handles POST /api/files/rename
func (h *Handler) RenameFile(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.NewName == "" {
		http.Error(w, "New name must not be empty", http.StatusBadRequest)
		return
	}
	if strings.Contains(req.NewName, "/") || strings.Contains(req.NewName, "..") {
		http.Error(w, "Invalid new name", http.StatusBadRequest)
		return
	}

	srcFull, err := h.roots.ResolvePath(req.RootID, req.Path)
	if err != nil {
		h.logger.Error("resolving path", "error", err, "rootID", req.RootID, "path", req.Path)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	dstFull := filepath.Join(filepath.Dir(srcFull), req.NewName)

	if err := os.Rename(srcFull, dstFull); err != nil {
		h.logger.Error("renaming file", "error", err, "src", srcFull, "dst", dstFull)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info("renamed", "rootID", req.RootID, "from", req.Path, "to", req.NewName)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"newName": req.NewName}); err != nil {
		slog.Error("encoding response", "error", err)
	}
}

func allErrors(results []itemResult) bool {
	for _, r := range results {
		if r.Error == "" {
			return false
		}
	}
	return true
}
