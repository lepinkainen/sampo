package server

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/lepinkainen/filemanager/internal/config"
	"github.com/lepinkainen/filemanager/internal/filesystem"
	"github.com/lepinkainen/filemanager/internal/server/handlers"
	"github.com/lepinkainen/filemanager/internal/thumbnail"
)

// Server is the main HTTP server.
type Server struct {
	cfg    *config.Config
	router *chi.Mux
	logger *slog.Logger
}

// New creates a new server instance.
func New(cfg *config.Config, frontendFS fs.FS, logger *slog.Logger) (*Server, error) {
	rootMgr, err := filesystem.NewRootManager(cfg.Roots)
	if err != nil {
		return nil, fmt.Errorf("initializing roots: %w", err)
	}

	thumbCache, err := thumbnail.NewCache(cfg.Cache.Dir + "/thumbs")
	if err != nil {
		return nil, fmt.Errorf("initializing thumbnail cache: %w", err)
	}

	s := &Server{
		cfg:    cfg,
		router: chi.NewRouter(),
		logger: logger,
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Compress(5))

	h := handlers.New(rootMgr, thumbCache, logger)
	s.setupRoutes(h, frontendFS)

	return s, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	s.logger.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, s.router)
}
