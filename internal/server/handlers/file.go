package handlers

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

// ServeFile serves the original file with proper Content-Type and Range support.
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
	rootID := chi.URLParam(r, "rootID")
	relPath := chi.URLParam(r, "*")

	fullPath, err := h.roots.ResolvePath(rootID, relPath)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(fullPath)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		http.Error(w, "Not a file", http.StatusBadRequest)
		return
	}

	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}
