package handlers

import (
	"log/slog"
	"sync/atomic"

	"github.com/lepinkainen/filemanager/internal/analysis"
	"github.com/lepinkainen/filemanager/internal/classification"
	"github.com/lepinkainen/filemanager/internal/detection"
	"github.com/lepinkainen/filemanager/internal/filesystem"
	"github.com/lepinkainen/filemanager/internal/thumbnail"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	roots             *filesystem.RootManager
	thumbCache        *thumbnail.Cache
	frameDir          string
	logger            *slog.Logger
	detectionStore    *detection.Store
	detector          *detection.Detector
	scanner           *detection.Scanner
	classStore        *classification.Store
	classifier        *classification.Classifier
	classScanner      *classification.Scanner
	browseCoordinator *analysis.Coordinator
	autoBrowseEnabled atomic.Bool
}

// New creates a new Handler.
func New(roots *filesystem.RootManager, thumbCache *thumbnail.Cache, frameDir string, logger *slog.Logger) *Handler {
	return &Handler{
		roots:      roots,
		thumbCache: thumbCache,
		frameDir:   frameDir,
		logger:     logger,
	}
}

// SetDetection configures optional detection components.
func (h *Handler) SetDetection(store *detection.Store, detector *detection.Detector, scanner *detection.Scanner) {
	h.detectionStore = store
	h.detector = detector
	h.scanner = scanner
}

// SetClassification configures optional classification components.
func (h *Handler) SetClassification(store *classification.Store, classifier *classification.Classifier, scanner *classification.Scanner) {
	h.classStore = store
	h.classifier = classifier
	h.classScanner = scanner
}

// SetBrowseCoordinator configures optional browse-triggered background analysis.
func (h *Handler) SetBrowseCoordinator(coordinator *analysis.Coordinator) {
	h.browseCoordinator = coordinator
}

// SetAutoBrowseEnabled updates the runtime browse-triggered analysis flag.
func (h *Handler) SetAutoBrowseEnabled(enabled bool) {
	h.autoBrowseEnabled.Store(enabled)
}

// AutoBrowseEnabled reports whether browse-triggered analysis is enabled.
func (h *Handler) AutoBrowseEnabled() bool {
	return h.autoBrowseEnabled.Load()
}
