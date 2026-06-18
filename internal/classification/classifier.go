package classification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"io"
	"log/slog"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	ort "github.com/yalue/onnxruntime_go"
)

const clipInputSize = 224

// CLIP normalization constants (ImageNet-based).
var (
	clipMean = [3]float32{0.48145466, 0.4578275, 0.40821073}
	clipStd  = [3]float32{0.26862954, 0.26130258, 0.27577711}
)

// LabelsFile is the JSON format for pre-computed label embeddings.
type LabelsFile struct {
	Model  string  `json:"model"`
	Dim    int     `json:"dim"`
	Labels []Label `json:"labels"`
}

// Label holds a pre-computed text embedding for a classification label.
type Label struct {
	Name      string    `json:"name"`
	Prompt    string    `json:"prompt"`
	Group     string    `json:"group,omitempty"`
	Embedding []float32 `json:"embedding"`
}

// TagScore holds a classification tag and its confidence score.
type TagScore struct {
	Label string  `json:"label"`
	Score float32 `json:"score"`
}

// Result holds the result of a classification.
type Result struct {
	RootID    string     `json:"rootId"`
	RelPath   string     `json:"relPath"`
	Mtime     int64      `json:"mtime"`
	Size      int64      `json:"size"`
	ModelVer  string     `json:"modelVer"`
	ScannedAt time.Time  `json:"scannedAt"`
	Tags      []TagScore `json:"tags"`
	SHA256    string     `json:"sha256,omitempty"`
	CRC32     string     `json:"crc32,omitempty"`
}

// Classifier runs CLIP image classification using ONNX Runtime.
type Classifier struct {
	mu           sync.Mutex
	session      *ort.AdvancedSession
	inputTensor  *ort.Tensor[float32]
	outputTensor *ort.Tensor[float32]
	labels       []Label
	threshold    float32
	modelVer     string
	logger       *slog.Logger
}

// NewClassifier loads the CLIP ONNX model and label embeddings.
// The caller must call onnxenv.Init() before creating a classifier.
func NewClassifier(modelPath, labelsPath string, threshold float32, modelVer string, logger *slog.Logger) (*Classifier, error) {
	// Load label embeddings
	labelsData, err := os.ReadFile(labelsPath)
	if err != nil {
		return nil, fmt.Errorf("reading labels file: %w", err)
	}

	var labelsFile LabelsFile
	err = json.Unmarshal(labelsData, &labelsFile)
	if err != nil {
		return nil, fmt.Errorf("parsing labels file: %w", err)
	}

	if len(labelsFile.Labels) == 0 {
		return nil, fmt.Errorf("no labels defined in %s", labelsPath)
	}

	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("creating session options: %w", err)
	}
	defer func() { _ = opts.Destroy() }()

	if runtime.GOOS == "darwin" {
		if coreMLErr := opts.AppendExecutionProviderCoreML(0); coreMLErr != nil {
			logger.Warn("CoreML not available for CLIP, using CPU", "error", coreMLErr)
		}
	}

	// CLIP ViT-B/32 input: [1, 3, 224, 224]
	inputShape := ort.NewShape(1, 3, clipInputSize, clipInputSize)
	inputTensor, err := ort.NewEmptyTensor[float32](inputShape)
	if err != nil {
		return nil, fmt.Errorf("creating input tensor: %w", err)
	}

	// CLIP output: [1, 512] (image embedding)
	outputShape := ort.NewShape(1, int64(labelsFile.Dim))
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		_ = inputTensor.Destroy()
		return nil, fmt.Errorf("creating output tensor: %w", err)
	}

	session, err := ort.NewAdvancedSession(modelPath,
		[]string{"pixel_values"}, []string{"image_embeds"},
		[]ort.ArbitraryTensor{inputTensor}, []ort.ArbitraryTensor{outputTensor},
		opts,
	)
	if err != nil {
		_ = inputTensor.Destroy()
		_ = outputTensor.Destroy()
		return nil, fmt.Errorf("creating CLIP ONNX session: %w", err)
	}

	return &Classifier{
		session:      session,
		inputTensor:  inputTensor,
		outputTensor: outputTensor,
		labels:       labelsFile.Labels,
		threshold:    threshold,
		modelVer:     modelVer,
		logger:       logger,
	}, nil
}

// computeFileHashes computes SHA256 and CRC32 for a file.
func computeFileHashes(path string) (sha256Hex, crc32Hex string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = f.Close() }()

	sha256Hash := sha256.New()
	crc32Hash := crc32.NewIEEE()
	w := io.MultiWriter(sha256Hash, crc32Hash)

	if _, err := io.Copy(w, f); err != nil {
		return "", "", err
	}

	return hex.EncodeToString(sha256Hash.Sum(nil)),
		strings.ToUpper(fmt.Sprintf("%08X", crc32Hash.Sum32())),
		nil
}

// Classify runs CLIP classification on an image file and returns tags above threshold.
func (c *Classifier) Classify(imagePath string, rootID, relPath string, mtime, size int64) (*Result, error) {
	// Compute hashes before locking (IO-bound, doesn't need ONNX mutex)
	sha256Hex, crc32Hex, hashErr := computeFileHashes(imagePath)
	if hashErr != nil {
		c.logger.Warn("failed to compute file hashes", "path", imagePath, "error", hashErr)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	img, err := imaging.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("opening image %s: %w", imagePath, err)
	}

	// CLIP preprocessing: resize shortest side to 224, center crop 224x224
	preprocessed := clipPreprocess(img)

	// Convert to CHW tensor with CLIP normalization
	inputData := clipImageToTensor(preprocessed)
	copy(c.inputTensor.GetData(), inputData)

	if err := c.session.Run(); err != nil {
		return nil, fmt.Errorf("running CLIP inference: %w", err)
	}

	embedding := c.outputTensor.GetData()

	// L2-normalize the image embedding
	normalized := l2Normalize(embedding)

	// Compute cosine similarity against each label embedding.
	// For grouped labels (e.g. mutually-exclusive attire), keep only the
	// highest-scoring tag per group. Ungrouped labels all pass through.
	var tags []TagScore
	bestByGroup := make(map[string]int) // group -> index into tags
	for _, label := range c.labels {
		score := cosineSimilarity(normalized, label.Embedding)
		if score < c.threshold {
			continue
		}
		if label.Group == "" {
			tags = append(tags, TagScore{Label: label.Name, Score: score})
			continue
		}
		if idx, ok := bestByGroup[label.Group]; ok {
			if score > tags[idx].Score {
				tags[idx] = TagScore{Label: label.Name, Score: score}
			}
			continue
		}
		bestByGroup[label.Group] = len(tags)
		tags = append(tags, TagScore{Label: label.Name, Score: score})
	}

	return &Result{
		RootID:    rootID,
		RelPath:   relPath,
		Mtime:     mtime,
		Size:      size,
		ModelVer:  c.modelVer,
		ScannedAt: time.Now(),
		Tags:      tags,
		SHA256:    sha256Hex,
		CRC32:     crc32Hex,
	}, nil
}

// ModelVersion returns the model version string.
func (c *Classifier) ModelVersion() string {
	return c.modelVer
}

// Destroy cleans up ONNX session resources.
func (c *Classifier) Destroy() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		_ = c.session.Destroy()
	}
	if c.inputTensor != nil {
		_ = c.inputTensor.Destroy()
	}
	if c.outputTensor != nil {
		_ = c.outputTensor.Destroy()
	}
}

// clipPreprocess resizes the shortest side to 224 and center crops to 224x224.
func clipPreprocess(img image.Image) *image.NRGBA {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Resize so shortest side is 224
	var resized *image.NRGBA
	if w < h {
		resized = imaging.Resize(img, clipInputSize, 0, imaging.Lanczos)
	} else {
		resized = imaging.Resize(img, 0, clipInputSize, imaging.Lanczos)
	}

	// Center crop to 224x224
	return imaging.CropCenter(resized, clipInputSize, clipInputSize)
}

// clipImageToTensor converts an NRGBA image to a CHW float32 slice with CLIP normalization.
func clipImageToTensor(img *image.NRGBA) []float32 {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	data := make([]float32, 3*w*h)

	for y := range h {
		for x := range w {
			offset := y*img.Stride + x*4
			r := float32(img.Pix[offset]) / 255.0
			g := float32(img.Pix[offset+1]) / 255.0
			b := float32(img.Pix[offset+2]) / 255.0

			// Apply CLIP normalization: (pixel - mean) / std
			data[0*w*h+y*w+x] = (r - clipMean[0]) / clipStd[0]
			data[1*w*h+y*w+x] = (g - clipMean[1]) / clipStd[1]
			data[2*w*h+y*w+x] = (b - clipMean[2]) / clipStd[2]
		}
	}
	return data
}

// l2Normalize returns a copy of the vector normalized to unit length.
func l2Normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sum))
	if norm == 0 {
		return v
	}
	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = x / norm
	}
	return result
}

// cosineSimilarity computes the dot product of two L2-normalized vectors.
func cosineSimilarity(a, b []float32) float32 {
	var dot float32
	for i := range a {
		if i < len(b) {
			dot += a[i] * b[i]
		}
	}
	return dot
}
