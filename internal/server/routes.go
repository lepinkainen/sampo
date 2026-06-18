package server

import (
	"io/fs"
	"net/http"

	"github.com/lepinkainen/sampo/internal/server/handlers"
)

func (s *Server) setupRoutes(h *handlers.Handler, frontendFS fs.FS) {
	// API routes
	s.router.Get("/whoami", h.WhoAmI)
	s.router.Get("/health", h.Health)
	s.router.Get("/api/roots", h.ListRoots)
	s.router.Get("/api/tree/{rootID}/*", h.ListDirectory)
	s.router.Get("/api/thumb/{rootID}/*", h.GetThumbnail)
	s.router.Get("/api/file/{rootID}/*", h.ServeFile)
	s.router.Get("/api/analysis/settings", h.GetAnalysisSettings)
	s.router.Post("/api/analysis/settings", h.UpdateAnalysisSettings)

	// File operations
	s.router.Delete("/api/files/{rootID}/*", h.DeleteFile)
	s.router.Post("/api/files/move", h.MoveFiles)
	s.router.Post("/api/files/copy", h.CopyFiles)
	s.router.Post("/api/files/rename", h.RenameFile)

	// Detection (returns 503 if not enabled)
	s.router.Get("/api/detect/{rootID}/*", h.DetectFile)
	s.router.Post("/api/detect/scan", h.StartScan)
	s.router.Get("/api/detect/status", h.ScanStatus)

	// Search
	s.router.Get("/api/search/{rootID}", h.SearchFiles)

	// Disk usage
	s.router.Get("/api/usage/{rootID}/*", h.GetDiskUsage)

	// Duplicates (returns 503 if classification not enabled)
	s.router.Get("/api/duplicates/{rootID}/*", h.FindDuplicates)

	// Classification (returns 503 if not enabled)
	s.router.Get("/api/classify/{rootID}/*", h.ClassifyFile)
	s.router.Post("/api/classify/scan", h.StartClassifyScan)
	s.router.Get("/api/classify/status", h.ClassifyScanStatus)

	// Serve frontend SPA from disk
	fileServer := http.FileServer(http.FS(frontendFS))
	s.router.Handle("/*", spaHandler(frontendFS, fileServer))
}

// spaHandler tries to serve static files, falling back to index.html for SPA routing.
func spaHandler(frontendFS fs.FS, fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try opening the requested path
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Strip leading slash for fs.Open
		fsPath := path[1:]
		f, err := frontendFS.Open(fsPath)
		if err != nil {
			// Not found — serve index.html for client-side routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		_ = f.Close()

		fileServer.ServeHTTP(w, r)
	})
}
