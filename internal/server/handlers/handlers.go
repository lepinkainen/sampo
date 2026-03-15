package handlers

import (
	"log/slog"

	"github.com/lepinkainen/filemanager/internal/filesystem"
	"github.com/lepinkainen/filemanager/internal/thumbnail"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	roots      *filesystem.RootManager
	thumbCache *thumbnail.Cache
	logger     *slog.Logger
}

// New creates a new Handler.
func New(roots *filesystem.RootManager, thumbCache *thumbnail.Cache, logger *slog.Logger) *Handler {
	return &Handler{
		roots:      roots,
		thumbCache: thumbCache,
		logger:     logger,
	}
}
