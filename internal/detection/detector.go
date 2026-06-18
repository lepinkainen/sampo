package detection

import (
	"fmt"
	"image"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	inputSize       = 640
	cocoPersonClass = 0
)

// Detector runs YOLO11 person detection using ONNX Runtime.
type Detector struct {
	mu           sync.Mutex
	session      *ort.AdvancedSession
	inputTensor  *ort.Tensor[float32]
	outputTensor *ort.Tensor[float32]
	threshold    float32
	modelVer     string
	logger       *slog.Logger
}

// NewDetector loads the ONNX model and creates a detector.
// The caller must call onnxenv.Init() before creating a detector.
func NewDetector(modelPath string, threshold float32, modelVer string, logger *slog.Logger) (*Detector, error) {
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("creating session options: %w", err)
	}
	defer func() { _ = opts.Destroy() }()

	// Enable CoreML on macOS
	if runtime.GOOS == "darwin" {
		if coreMLErr := opts.AppendExecutionProviderCoreML(0); coreMLErr != nil {
			logger.Warn("CoreML not available, using CPU", "error", coreMLErr)
		}
	}

	// Define input/output shapes for YOLO11n
	inputShape := ort.NewShape(1, 3, inputSize, inputSize)
	inputTensor, err := ort.NewEmptyTensor[float32](inputShape)
	if err != nil {
		return nil, fmt.Errorf("creating input tensor: %w", err)
	}

	// YOLO11 output: [1, 84, 8400] — 84 = 4 bbox + 80 classes, 8400 detections
	outputShape := ort.NewShape(1, 84, 8400)
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		_ = inputTensor.Destroy()
		return nil, fmt.Errorf("creating output tensor: %w", err)
	}

	session, err := ort.NewAdvancedSession(modelPath,
		[]string{"images"}, []string{"output0"},
		[]ort.ArbitraryTensor{inputTensor}, []ort.ArbitraryTensor{outputTensor},
		opts,
	)
	if err != nil {
		_ = inputTensor.Destroy()
		_ = outputTensor.Destroy()
		return nil, fmt.Errorf("creating ONNX session: %w", err)
	}

	return &Detector{
		session:      session,
		inputTensor:  inputTensor,
		outputTensor: outputTensor,
		threshold:    threshold,
		modelVer:     modelVer,
		logger:       logger,
	}, nil
}

// Detect runs person detection on an image file.
func (d *Detector) Detect(imagePath string, rootID, relPath string, mtime int64, size int64) (*Result, error) {
	img, err := imaging.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("opening image %s: %w", imagePath, err)
	}
	return d.DetectImage(img, rootID, relPath, mtime, size)
}

// DetectImage runs person detection on an already-decoded image. Callers that
// run several analyzers on the same file decode it once and share it here,
// avoiding a redundant per-analyzer file read + decode.
func (d *Detector) DetectImage(img image.Image, rootID, relPath string, mtime int64, size int64) (*Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Resize to 640x640 maintaining aspect ratio with letterboxing
	resized := imaging.Fit(img, inputSize, inputSize, imaging.Lanczos)
	// Paste onto a black 640x640 canvas
	canvas := imaging.New(inputSize, inputSize, image.Black)
	offsetX := (inputSize - resized.Bounds().Dx()) / 2
	offsetY := (inputSize - resized.Bounds().Dy()) / 2
	canvas = imaging.Paste(canvas, resized, image.Pt(offsetX, offsetY))

	// Convert to CHW float32 tensor [1, 3, 640, 640], normalized to [0, 1]
	inputData := imageToTensor(canvas)

	// Copy preprocessed image data into input tensor
	copy(d.inputTensor.GetData(), inputData)

	if err := d.session.Run(); err != nil {
		return nil, fmt.Errorf("running inference: %w", err)
	}

	outputData := d.outputTensor.GetData()

	hasPerson, confidence := d.parseYOLOOutput(outputData)

	return &Result{
		RootID:     rootID,
		RelPath:    relPath,
		Mtime:      mtime,
		Size:       size,
		HasPerson:  hasPerson,
		Confidence: float64(confidence),
		ModelVer:   d.modelVer,
		ScannedAt:  time.Now(),
	}, nil
}

// parseYOLOOutput parses YOLO11 output [1, 84, 8400] for person detections.
// Each of 8400 detections has 84 values (4 bbox coords + 80 class scores),
// stored transposed as 84 rows of 8400. Class 0 = person. YOLO11 keeps the
// same output layout as YOLOv8, so this parser is unchanged across the upgrade.
func (d *Detector) parseYOLOOutput(output []float32) (hasPerson bool, maxConf float32) {
	const numDetections = 8400
	const personIdx = 4 + cocoPersonClass // bbox(4) + person class(0)

	for i := 0; i < numDetections; i++ {
		// YOLO11 output is [1, 84, 8400], stored as 84 rows of 8400
		personScore := output[personIdx*numDetections+i]
		if personScore > d.threshold && personScore > maxConf {
			maxConf = personScore
			hasPerson = true
		}
	}
	return
}

// imageToTensor converts an NRGBA image to a CHW float32 slice normalized to [0, 1].
func imageToTensor(img *image.NRGBA) []float32 {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	data := make([]float32, 3*w*h)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			offset := y*img.Stride + x*4
			r := float32(img.Pix[offset]) / 255.0
			g := float32(img.Pix[offset+1]) / 255.0
			b := float32(img.Pix[offset+2]) / 255.0

			data[0*w*h+y*w+x] = r
			data[1*w*h+y*w+x] = g
			data[2*w*h+y*w+x] = b
		}
	}
	return data
}

// ModelVersion returns the model version string.
func (d *Detector) ModelVersion() string {
	return d.modelVer
}

// Destroy cleans up ONNX session resources.
// Does not destroy the shared ONNX environment — that is managed by onnxenv.
func (d *Detector) Destroy() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.session != nil {
		_ = d.session.Destroy()
	}
	if d.inputTensor != nil {
		_ = d.inputTensor.Destroy()
	}
	if d.outputTensor != nil {
		_ = d.outputTensor.Destroy()
	}
}
