package handlers

import (
	"net/http"
	"net/url"
	"os"

	"github.com/go-chi/chi/v5"
)

// ServeFile serves the original file with proper Content-Type and Range support.
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
	rootID := chi.URLParam(r, "rootID")
	relPath, err := url.PathUnescape(chi.URLParam(r, "*"))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

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
