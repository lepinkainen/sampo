package server

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/lepinkainen/sampo/internal/analysis"
	"github.com/lepinkainen/sampo/internal/classification"
	"github.com/lepinkainen/sampo/internal/config"
	"github.com/lepinkainen/sampo/internal/detection"
	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/ocr"
	"github.com/lepinkainen/sampo/internal/onnxenv"
	"github.com/lepinkainen/sampo/internal/server/handlers"
	"github.com/lepinkainen/sampo/internal/thumbnail"
	"github.com/lepinkainen/sampo/internal/videoframe"
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

	// Prune stale thumbnails from cache
	if cfg.Cache.MaxAgeDays > 0 {
		maxAge := time.Duration(cfg.Cache.MaxAgeDays) * 24 * time.Hour
		pruned, pruneErr := thumbCache.Prune(maxAge)
		if pruneErr != nil {
			logger.Warn("pruning thumbnail cache", "error", pruneErr)
		}
		if pruned > 0 {
			logger.Info("pruned stale thumbnails", "count", pruned)
		}
	}

	// Clean leftover video frames from previous runs
	frameDir := cfg.Cache.Dir + "/frames"
	if cleanErr := videoframe.CleanDir(frameDir); cleanErr != nil {
		logger.Warn("cleaning leftover video frames", "error", cleanErr)
	}

	s := &Server{
		cfg:    cfg,
		router: chi.NewRouter(),
		logger: logger,
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Compress(5))

	h := handlers.New(rootMgr, thumbCache, frameDir, logger)
	h.SetAutoBrowseEnabled(cfg.Analysis.AutoBrowseEnabled)

	var detStore *detection.Store
	var detector *detection.Detector
	var classStore *classification.Store
	var classifier *classification.Classifier
	var ocrStore *ocr.Store
	var recognizer *ocr.Recognizer

	// Initialize shared ONNX environment if any ML feature is enabled.
	// OCR's in-process backend (Linux/Docker) is ONNX too; the darwin Vision
	// backend doesn't need ORT, but onnxenv.Init is idempotent and the OCR engine
	// also calls it defensively, so gating on OCR.Enabled here is harmless on macOS.
	if cfg.Detection.Enabled || cfg.Classification.Enabled || cfg.OCR.Enabled {
		if initErr := onnxenv.Init(); initErr != nil {
			return nil, fmt.Errorf("initializing ONNX Runtime: %w", initErr)
		}
	}

	// Conditionally initialize detection
	if cfg.Detection.Enabled {
		detStore, err = detection.NewStore(cfg.Cache.Dir)
		if err != nil {
			return nil, fmt.Errorf("initializing detection store: %w", err)
		}

		detector, err = detection.NewDetector(
			cfg.Detection.ModelPath,
			cfg.Detection.Threshold,
			cfg.Detection.ModelVersion,
			logger,
		)
		if err != nil {
			_ = detStore.Close()
			return nil, fmt.Errorf("initializing detector: %w", err)
		}

		scanner := detection.NewScanner(detStore, detector, rootMgr, frameDir, cfg.Detection.Workers, logger)
		h.SetDetection(detStore, detector, scanner)
		logger.Info("person detection enabled", "model", cfg.Detection.ModelPath, "threshold", cfg.Detection.Threshold)
	}

	// Conditionally initialize classification
	if cfg.Classification.Enabled {
		classStore, err = classification.NewStore(cfg.Cache.Dir)
		if err != nil {
			return nil, fmt.Errorf("initializing classification store: %w", err)
		}

		classifier, err = classification.NewClassifier(
			cfg.Classification.ModelPath,
			cfg.Classification.LabelsPath,
			cfg.Classification.Threshold,
			cfg.Classification.ModelVersion,
			logger,
		)
		if err != nil {
			_ = classStore.Close()
			return nil, fmt.Errorf("initializing classifier: %w", err)
		}

		classScanner := classification.NewScanner(classStore, classifier, rootMgr, frameDir, cfg.Classification.Workers, logger)
		h.SetClassification(classStore, classifier, classScanner)
		logger.Info("classification enabled", "model", cfg.Classification.ModelPath, "threshold", cfg.Classification.Threshold)
	}

	// Conditionally initialize OCR. The backend is platform-gated in
	// internal/ocr (Vision binary on macOS, unsupported elsewhere for now), so a
	// recognizer build failure disables the feature instead of killing startup.
	if cfg.OCR.Enabled {
		rec, ocrErr := ocr.NewRecognizer(ocr.Options{
			BinaryPath:   cfg.OCR.BinaryPath,
			ModelVersion: cfg.OCR.ModelVersion,
			DetModelPath: cfg.OCR.DetModelPath,
			RecModelPath: cfg.OCR.RecModelPath,
			DictPath:     cfg.OCR.DictPath,
		}, logger)
		if ocrErr != nil {
			logger.Warn("OCR enabled but unavailable on this platform, skipping", "error", ocrErr)
		} else {
			store, storeErr := ocr.NewStore(cfg.Cache.Dir)
			if storeErr != nil {
				return nil, fmt.Errorf("initializing ocr store: %w", storeErr)
			}
			ocrStore = store
			recognizer = rec
			ocrScanner := ocr.NewScanner(ocrStore, recognizer, rootMgr, frameDir, cfg.OCR.Workers, logger)
			h.SetOCR(ocrStore, recognizer, ocrScanner)
			logger.Info("OCR enabled", "binary", cfg.OCR.BinaryPath, "version", recognizer.ModelVersion())
		}
	}

	if cfg.Detection.Enabled || cfg.Classification.Enabled || ocrStore != nil {
		coordinator := analysis.NewCoordinator(
			detStore,
			detector,
			classStore,
			classifier,
			ocrStore,
			recognizer,
			frameDir,
			cfg.Analysis.BrowseWorkers,
			cfg.Analysis.BrowseQueueSize,
			cfg.Analysis.IncludeVideos,
			logger,
		)
		h.SetBrowseCoordinator(coordinator)

		// Unified scan: one walk, every enabled analyzer per file. Re-analyze uses this.
		analysisScanner := analysis.NewScanner(coordinator, rootMgr, cfg.Analysis.BrowseWorkers, logger)
		h.SetAnalysisScanner(analysisScanner)

		logger.Info("browse analysis configured",
			"autoEnabled", cfg.Analysis.AutoBrowseEnabled,
			"workers", cfg.Analysis.BrowseWorkers,
			"queueSize", cfg.Analysis.BrowseQueueSize,
			"includeVideos", cfg.Analysis.IncludeVideos,
		)
	}

	s.setupRoutes(h, frontendFS)

	return s, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	s.logger.Info("starting server", "addr", addr)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return httpServer.ListenAndServe()
}
