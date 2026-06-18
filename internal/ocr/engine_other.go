//go:build !darwin

package ocr

import (
	"fmt"
	"log/slog"
)

// newEngine has no OCR backend on non-darwin platforms yet.
//
// TODO(linux): plug in a generic ONNX backend here — RapidOCR/PaddleOCR
// (DBNet text detection + SVTR/CRNN recognition, both exportable to ONNX) is
// the planned path, reusing internal/onnxenv like detection/classification.
// Until then OCR self-disables: the server logs a warning and skips wiring.
//
// When implemented, the backend's Recognize MUST use the shared decoded image
// (the `img image.Image` argument) when it is non-nil, rather than re-reading
// imagePath from disk — the unified pipeline decodes each file once and hands
// the same in-memory image to detection, classification, and OCR. Only fall
// back to decoding imagePath when img is nil (standalone single-file callers).
func newEngine(binaryPath, modelVersion string, logger *slog.Logger) (Engine, error) {
	return nil, fmt.Errorf("%w (no generic backend wired yet)", ErrUnsupported)
}
