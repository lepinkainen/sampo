//go:build !darwin

package ocr

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/lepinkainen/sampo/internal/onnxenv"
	ort "github.com/yalue/onnxruntime_go"
)

// In-process ONNX OCR backend for Linux/Docker, implementing the PP-OCR
// (RapidOCR) two-stage pipeline:
//
//   - detection: a DBNet model emits a per-pixel text-probability map. We
//     binarize it, group pixels into connected components, take each
//     component's axis-aligned bounding box, score it, and "unclip" (dilate)
//     it back toward the original glyph extent.
//   - recognition: each detected box is cropped, resized to height 48, and run
//     through a CRNN/SVTR model whose output is CTC-decoded into text.
//
// Both models have fully dynamic input shapes, so we use DynamicAdvancedSession
// and create a per-call input tensor; outputs are auto-allocated by Run().
//
// Known limitation (MVP): detection uses axis-aligned boxes rather than the
// rotated min-area rectangles PaddleOCR/OpenCV produce, so heavily skewed text
// is not deskewed before recognition. This is adequate for the screenshot/photo
// text search this feature targets and leaves room for a rotated-rect upgrade.
const (
	detLimitSide   = 960  // longest side fed to the detector (rounded to /32)
	detBinThresh   = 0.3  // probability threshold for the detection bitmap
	detBoxThresh   = 0.6  // minimum mean-probability for a kept box
	detUnclipRatio = 1.6  // box dilation factor
	detMinBoxSide  = 3    // drop boxes smaller than this (detector pixels)
	recImageHeight = 48   // recognition input height
	recMaxWidth    = 1280 // cap recognition input width (long lines truncate)
	recMinConf     = 0.5  // drop recognized lines below this confidence
)

// onnxEngine runs the PP-OCR detection + recognition models via ONNX Runtime.
type onnxEngine struct {
	mu      sync.Mutex // det/rec sessions are not safe for concurrent Run
	det     *ort.DynamicAdvancedSession
	rec     *ort.DynamicAdvancedSession
	charset []string // CTC index -> string; index 0 is the blank ("")
	version string
	logger  *slog.Logger
}

// newEngine builds the ONNX PP-OCR backend. Requires det/rec model paths and a
// recognition char dictionary; returns ErrUnsupported (wrapped) if unset so the
// server self-disables OCR rather than failing startup.
func newEngine(opts Options, logger *slog.Logger) (Engine, error) {
	if opts.DetModelPath == "" || opts.RecModelPath == "" || opts.DictPath == "" {
		return nil, fmt.Errorf("%w: det_model_path, rec_model_path and dict_path are required", ErrUnsupported)
	}
	for _, p := range []string{opts.DetModelPath, opts.RecModelPath, opts.DictPath} {
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("ocr model file %q: %w", p, err)
		}
	}
	if err := onnxenv.Init(); err != nil {
		return nil, fmt.Errorf("initializing ONNX Runtime: %w", err)
	}

	charset, err := loadDict(opts.DictPath)
	if err != nil {
		return nil, err
	}

	det, err := newSession(opts.DetModelPath)
	if err != nil {
		return nil, fmt.Errorf("loading detection model: %w", err)
	}
	rec, err := newSession(opts.RecModelPath)
	if err != nil {
		_ = det.Destroy()
		return nil, fmt.Errorf("loading recognition model: %w", err)
	}

	version := opts.ModelVersion
	if version == "" {
		version = "onnx-ppocrv4-1.0"
	}
	return &onnxEngine{
		det:     det,
		rec:     rec,
		charset: charset,
		version: "linux-" + version,
		logger:  logger,
	}, nil
}

// newSession opens a single-input/single-output dynamic session, reading the
// input/output names from the model so we don't hard-code PaddleOCR's tensor
// names (which differ between det and rec).
func newSession(modelPath string) (*ort.DynamicAdvancedSession, error) {
	inputs, outputs, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return nil, fmt.Errorf("reading model io info: %w", err)
	}
	if len(inputs) == 0 || len(outputs) == 0 {
		return nil, fmt.Errorf("model %q has no inputs/outputs", modelPath)
	}
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("creating session options: %w", err)
	}
	defer func() { _ = opts.Destroy() }()
	return ort.NewDynamicAdvancedSession(modelPath,
		[]string{inputs[0].Name}, []string{outputs[0].Name}, opts)
}

// loadDict builds the CTC index->char table: blank at index 0, then each line of
// the dictionary, then a trailing space (PaddleOCR's use_space_char convention).
func loadDict(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading ocr dict %q: %w", path, err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	// Drop a single trailing empty element from a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	charset := make([]string, 0, len(lines)+2)
	charset = append(charset, "") // index 0: CTC blank
	charset = append(charset, lines...)
	charset = append(charset, " ")
	return charset, nil
}

// Version returns the backend + model identifier used for cache invalidation.
func (e *onnxEngine) Version() string { return e.version }

// Destroy releases the ONNX sessions (not the shared environment).
func (e *onnxEngine) Destroy() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.det != nil {
		_ = e.det.Destroy()
	}
	if e.rec != nil {
		_ = e.rec.Destroy()
	}
}

// Recognize runs detection + recognition on the shared decoded image (or decodes
// imagePath when img is nil), returning one TextBlock per recognized line.
func (e *onnxEngine) Recognize(_ context.Context, img image.Image, imagePath string) ([]TextBlock, error) {
	if img == nil {
		opened, err := imaging.Open(imagePath, imaging.AutoOrientation(true))
		if err != nil {
			return nil, fmt.Errorf("opening image %q: %w", imagePath, err)
		}
		img = opened
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	bounds := img.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()
	if origW == 0 || origH == 0 {
		return nil, nil
	}

	boxes, err := e.detect(img, origW, origH)
	if err != nil {
		return nil, err
	}

	blocks := make([]TextBlock, 0, len(boxes))
	for _, box := range boxes {
		crop := imaging.Crop(img, box.Add(bounds.Min))
		text, conf, recErr := e.recognize(crop)
		if recErr != nil {
			return nil, recErr
		}
		if text == "" || conf < recMinConf {
			continue
		}
		blocks = append(blocks, TextBlock{
			Text:       text,
			Confidence: conf,
			X:          float32(box.Min.X) / float32(origW),
			Y:          float32(box.Min.Y) / float32(origH),
			W:          float32(box.Dx()) / float32(origW),
			H:          float32(box.Dy()) / float32(origH),
		})
	}
	return blocks, nil
}

// detect runs the DBNet detector and returns text boxes in original-image pixels.
func (e *onnxEngine) detect(img image.Image, origW, origH int) ([]image.Rectangle, error) {
	// Resize so the longest side <= detLimitSide, both dims rounded to /32.
	ratio := 1.0
	if longest := max(origW, origH); longest > detLimitSide {
		ratio = float64(detLimitSide) / float64(longest)
	}
	rw := roundTo32(float64(origW) * ratio)
	rh := roundTo32(float64(origH) * ratio)

	resized := imaging.Resize(img, rw, rh, imaging.Linear)
	data := make([]float32, 3*rh*rw)
	mean := [3]float32{0.485, 0.456, 0.406}
	std := [3]float32{0.229, 0.224, 0.225}
	plane := rh * rw
	for y := 0; y < rh; y++ {
		for x := 0; x < rw; x++ {
			o := y*resized.Stride + x*4
			idx := y*rw + x
			data[idx] = (float32(resized.Pix[o])/255 - mean[0]) / std[0]
			data[plane+idx] = (float32(resized.Pix[o+1])/255 - mean[1]) / std[1]
			data[2*plane+idx] = (float32(resized.Pix[o+2])/255 - mean[2]) / std[2]
		}
	}

	in, err := ort.NewTensor(ort.NewShape(1, 3, int64(rh), int64(rw)), data)
	if err != nil {
		return nil, fmt.Errorf("creating det input tensor: %w", err)
	}
	defer func() { _ = in.Destroy() }()

	outputs := []ort.Value{nil}
	if runErr := e.det.Run([]ort.Value{in}, outputs); runErr != nil {
		return nil, fmt.Errorf("running detection: %w", runErr)
	}
	out := outputs[0]
	defer func() { _ = out.Destroy() }()
	probTensor, ok := out.(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected detection output type %T", out)
	}
	shape := probTensor.GetShape()
	if len(shape) != 4 {
		return nil, fmt.Errorf("unexpected detection output shape %v", shape)
	}
	oh, ow := int(shape[2]), int(shape[3])

	scaleX := float64(origW) / float64(ow)
	scaleY := float64(origH) / float64(oh)
	return extractBoxes(probTensor.GetData(), oh, ow, origW, origH, scaleX, scaleY), nil
}

// extractBoxes binarizes the probability map, groups pixels into connected
// components (8-connectivity), filters by mean score and minimum size, unclips
// each box, and maps it back to original-image coordinates.
func extractBoxes(prob []float32, oh, ow, origW, origH int, scaleX, scaleY float64) []image.Rectangle {
	visited := make([]bool, oh*ow)
	stack := make([]int, 0, 256)
	var boxes []image.Rectangle

	for start := 0; start < oh*ow; start++ {
		if visited[start] || prob[start] <= detBinThresh {
			continue
		}
		// Flood-fill this component, tracking bbox and probability sum.
		minX, minY, maxX, maxY := ow, oh, -1, -1
		var sum float64
		count := 0
		stack = stack[:0]
		stack = append(stack, start)
		visited[start] = true
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			px, py := p%ow, p/ow
			if px < minX {
				minX = px
			}
			if px > maxX {
				maxX = px
			}
			if py < minY {
				minY = py
			}
			if py > maxY {
				maxY = py
			}
			sum += float64(prob[p])
			count++
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := px+dx, py+dy
					if nx < 0 || ny < 0 || nx >= ow || ny >= oh {
						continue
					}
					np := ny*ow + nx
					if visited[np] || prob[np] <= detBinThresh {
						continue
					}
					visited[np] = true
					stack = append(stack, np)
				}
			}
		}

		w := maxX - minX + 1
		h := maxY - minY + 1
		if w < detMinBoxSide || h < detMinBoxSide {
			continue
		}
		if sum/float64(count) < detBoxThresh {
			continue
		}

		// Unclip: dilate the box by distance = area*ratio/perimeter.
		area := float64(w * h)
		perim := 2.0 * float64(w+h)
		d := area * detUnclipRatio / perim
		fx0 := (float64(minX) - d) * scaleX
		fy0 := (float64(minY) - d) * scaleY
		fx1 := (float64(maxX+1) + d) * scaleX
		fy1 := (float64(maxY+1) + d) * scaleY
		x0 := clampInt(int(math.Floor(fx0)), 0, origW)
		y0 := clampInt(int(math.Floor(fy0)), 0, origH)
		x1 := clampInt(int(math.Ceil(fx1)), 0, origW)
		y1 := clampInt(int(math.Ceil(fy1)), 0, origH)
		if x1-x0 < detMinBoxSide || y1-y0 < detMinBoxSide {
			continue
		}
		boxes = append(boxes, image.Rect(x0, y0, x1, y1))
	}
	return boxes
}

// recognize crops nothing (crop is already a line image), resizes to height 48,
// runs the CRNN model, and CTC-decodes the output. Returns text + mean
// per-character confidence.
func (e *onnxEngine) recognize(crop image.Image) (text string, conf float32, err error) {
	b := crop.Bounds()
	cw, ch := b.Dx(), b.Dy()
	if cw == 0 || ch == 0 {
		return "", 0, nil
	}
	rw := int(math.Ceil(float64(recImageHeight) * float64(cw) / float64(ch)))
	rw = clampInt(rw, recImageHeight/2, recMaxWidth)

	resized := imaging.Resize(crop, rw, recImageHeight, imaging.Linear)
	data := make([]float32, 3*recImageHeight*rw)
	plane := recImageHeight * rw
	for y := 0; y < recImageHeight; y++ {
		for x := 0; x < rw; x++ {
			o := y*resized.Stride + x*4
			idx := y*rw + x
			data[idx] = float32(resized.Pix[o])/127.5 - 1
			data[plane+idx] = float32(resized.Pix[o+1])/127.5 - 1
			data[2*plane+idx] = float32(resized.Pix[o+2])/127.5 - 1
		}
	}

	in, err := ort.NewTensor(ort.NewShape(1, 3, int64(recImageHeight), int64(rw)), data)
	if err != nil {
		return "", 0, fmt.Errorf("creating rec input tensor: %w", err)
	}
	defer func() { _ = in.Destroy() }()

	outputs := []ort.Value{nil}
	if runErr := e.rec.Run([]ort.Value{in}, outputs); runErr != nil {
		return "", 0, fmt.Errorf("running recognition: %w", runErr)
	}
	out := outputs[0]
	defer func() { _ = out.Destroy() }()
	logits, ok := out.(*ort.Tensor[float32])
	if !ok {
		return "", 0, fmt.Errorf("unexpected recognition output type %T", out)
	}
	shape := logits.GetShape()
	if len(shape) != 3 {
		return "", 0, fmt.Errorf("unexpected recognition output shape %v", shape)
	}
	steps, classes := int(shape[1]), int(shape[2])
	if classes != len(e.charset) {
		return "", 0, fmt.Errorf("recognition classes %d != dict size %d (wrong dict?)", classes, len(e.charset))
	}
	text, conf = ctcDecode(logits.GetData(), steps, classes, e.charset)
	return text, conf, nil
}

// ctcDecode greedily decodes a [T, C] softmax map: per timestep take the argmax,
// drop blanks (index 0) and repeated indices, and average the kept probabilities.
func ctcDecode(data []float32, steps, classes int, charset []string) (text string, conf float32) {
	var sb strings.Builder
	var confSum float64
	kept := 0
	prev := -1
	for t := 0; t < steps; t++ {
		base := t * classes
		best := 0
		bestP := data[base]
		for c := 1; c < classes; c++ {
			if p := data[base+c]; p > bestP {
				bestP = p
				best = c
			}
		}
		if best != 0 && best != prev {
			sb.WriteString(charset[best])
			confSum += float64(bestP)
			kept++
		}
		prev = best
	}
	if kept == 0 {
		return "", 0
	}
	return strings.TrimSpace(sb.String()), float32(confSum / float64(kept))
}

// roundTo32 rounds a dimension to the nearest positive multiple of 32.
func roundTo32(v float64) int {
	r := int(math.Round(v/32.0)) * 32
	if r < 32 {
		r = 32
	}
	return r
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
