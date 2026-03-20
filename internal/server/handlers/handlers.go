package handlers

import (
	"log/slog"

	"github.com/lepinkainen/filemanager/internal/detection"
	"github.com/lepinkainen/filemanager/internal/filesystem"
	"github.com/lepinkainen/filemanager/internal/thumbnail"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	roots          *filesystem.RootManager
	thumbCache     *thumbnail.Cache
	logger         *slog.Logger
	detectionStore *detection.Store
	detector       *detection.Detector
	scanner        *detection.Scanner
}

// New creates a new Handler.
func New(roots *filesystem.RootManager, thumbCache *thumbnail.Cache, logger *slog.Logger) *Handler {
	return &Handler{
		roots:      roots,
		thumbCache: thumbCache,
		logger:     logger,
	}
}

// SetDetection configures optional detection components.
func (h *Handler) SetDetection(store *detection.Store, detector *detection.Detector, scanner *detection.Scanner) {
	h.detectionStore = store
	h.detector = detector
	h.scanner = scanner
}
