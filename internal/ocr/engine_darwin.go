//go:build darwin

package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

// defaultBinaryName is the Vision-framework OCR helper built from
// scripts/sampo-ocr.swift (see Taskfile `build-ocr`).
const defaultBinaryName = "sampo-ocr"

// recognizeTimeout caps a single image's OCR run.
const recognizeTimeout = 60 * time.Second

// visionEngine shells out to the sampo-ocr Swift binary, which uses Apple's
// Vision framework. Same exec-a-binary pattern as the ffmpeg thumbnail path.
type visionEngine struct {
	binaryPath string
	version    string
	logger     *slog.Logger
}

// newEngine resolves the sampo-ocr binary and returns a Vision-backed engine.
func newEngine(opts Options, logger *slog.Logger) (Engine, error) {
	resolved, err := resolveBinary(opts.BinaryPath)
	if err != nil {
		return nil, err
	}
	version := opts.ModelVersion
	if version == "" {
		version = "vision-1.0"
	}
	return &visionEngine{
		binaryPath: resolved,
		version:    "darwin-" + version,
		logger:     logger,
	}, nil
}

// resolveBinary uses the configured path if it exists, otherwise looks the
// binary up on PATH.
func resolveBinary(binaryPath string) (string, error) {
	if binaryPath != "" {
		if _, err := os.Stat(binaryPath); err == nil {
			return binaryPath, nil
		}
	}
	found, err := exec.LookPath(defaultBinaryName)
	if err != nil {
		return "", fmt.Errorf("%s binary not found (build it with `task build-ocr`): %w", defaultBinaryName, err)
	}
	return found, nil
}

// Version returns the backend + model identifier.
func (e *visionEngine) Version() string { return e.version }

// Recognize runs the sampo-ocr binary on imagePath and parses its JSON output.
// The Vision backend is a subprocess that reads the file by path, so the shared
// decoded image is not usable here and is ignored.
func (e *visionEngine) Recognize(ctx context.Context, _ image.Image, imagePath string) ([]TextBlock, error) {
	ctx, cancel := context.WithTimeout(ctx, recognizeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.binaryPath, imagePath)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("sampo-ocr failed: %w\nstderr: %s", err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("running sampo-ocr: %w", err)
	}

	var blocks []TextBlock
	if err := json.Unmarshal(out, &blocks); err != nil {
		return nil, fmt.Errorf("parsing sampo-ocr output: %w", err)
	}
	return blocks, nil
}
