// Package ocr extracts text from images for full-text search.
//
// The recognition backend is selected at compile time via build tags so each
// platform only pulls in the dependencies it can satisfy:
//
//   - darwin (engine_darwin.go): shells out to the Vision-framework `sampo-ocr`
//     binary (built from scripts/sampo-ocr.swift). Efficient, on-device, no models.
//   - other  (engine_other.go): returns ErrUnsupported for now. This is the seam
//     where a generic ONNX backend (e.g. RapidOCR/PaddleOCR) plugs in for Linux.
//
// Everything outside the Engine — the Recognizer wrapper, Store, and Scanner —
// is platform-independent and mirrors the classification package.
package ocr

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"path"
	"strings"
	"time"
)

// ErrUnsupported is returned by NewEngine on platforms without an OCR backend.
var ErrUnsupported = errors.New("ocr not supported on this platform")

// TextBlock is a single recognized line/region with a normalized bounding box.
type TextBlock struct {
	Text       string  `json:"text"`
	Confidence float32 `json:"confidence"`
	X          float32 `json:"x"`
	Y          float32 `json:"y"`
	W          float32 `json:"w"`
	H          float32 `json:"h"`
}

// Result holds OCR output for a single file.
type Result struct {
	RootID    string      `json:"rootId"`
	RelPath   string      `json:"relPath"`
	Mtime     int64       `json:"mtime"`
	Size      int64       `json:"size"`
	ModelVer  string      `json:"modelVer"`
	ScannedAt time.Time   `json:"scannedAt"`
	Text      string      `json:"text"`
	Blocks    []TextBlock `json:"blocks"`
}

// NormalizeRelPath returns the canonical OCR cache key for a root-relative path.
// OCR rows never keep a leading slash; the root directory itself is "".
func NormalizeRelPath(relPath string) string {
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	if relPath == "" || relPath == "." || relPath == "/" {
		return ""
	}
	cleaned := path.Clean("/" + relPath)
	if cleaned == "/" || cleaned == "." {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

// Engine is a platform-specific OCR backend. Implementations live in
// engine_darwin.go / engine_other.go and are constructed via NewEngine.
type Engine interface {
	// Recognize returns the text blocks found in an image.
	//
	// img is the already-decoded image when the caller has one (e.g. the unified
	// analysis pipeline, which decodes each file once and shares it across every
	// analyzer); it is nil for standalone single-file/scan callers. imagePath is
	// always the path to the source file.
	//
	// In-process backends (the ONNX/Docker backend) MUST prefer img when non-nil
	// to reuse the shared decode and avoid re-reading the file. The macOS backend
	// is a subprocess that can only take a path, so it ignores img and re-reads —
	// an accepted exception, since it cannot share an in-memory image anyway.
	Recognize(ctx context.Context, img image.Image, imagePath string) ([]TextBlock, error)
	// Version identifies the backend + model for cache invalidation.
	Version() string
}

// Recognizer wraps a platform Engine with hashing and Result assembly, mirroring
// classification.Classifier so the scanner and handlers stay symmetric.
type Recognizer struct {
	engine   Engine
	modelVer string
	logger   *slog.Logger
}

// NewRecognizer builds the platform OCR engine and wraps it. Returns
// ErrUnsupported (wrapped) on platforms without a backend.
func NewRecognizer(binaryPath, modelVersion string, logger *slog.Logger) (*Recognizer, error) {
	engine, err := newEngine(binaryPath, modelVersion, logger)
	if err != nil {
		return nil, err
	}
	return &Recognizer{
		engine:   engine,
		modelVer: engine.Version(),
		logger:   logger,
	}, nil
}

// ModelVersion returns the backend version string used for staleness checks.
func (r *Recognizer) ModelVersion() string {
	return r.modelVer
}

// Recognize runs OCR and assembles a Result. img is the shared decoded image
// when the caller has one (the unified pipeline) or nil (standalone callers);
// imagePath is always the source file path. The engine decides which to use.
func (r *Recognizer) Recognize(ctx context.Context, img image.Image, imagePath, rootID, relPath string, mtime, size int64) (*Result, error) {
	blocks, err := r.engine.Recognize(ctx, img, imagePath)
	if err != nil {
		return nil, fmt.Errorf("ocr recognize %s: %w", imagePath, err)
	}

	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if t := strings.TrimSpace(b.Text); t != "" {
			parts = append(parts, t)
		}
	}

	return &Result{
		RootID:    rootID,
		RelPath:   NormalizeRelPath(relPath),
		Mtime:     mtime,
		Size:      size,
		ModelVer:  r.modelVer,
		ScannedAt: time.Now(),
		Text:      strings.Join(parts, "\n"),
		Blocks:    blocks,
	}, nil
}
