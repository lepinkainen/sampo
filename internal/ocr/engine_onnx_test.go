//go:build !darwin

package ocr

import (
	"context"
	"image"
	"image/color"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func TestLoadDict(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "dict.txt")
	// Three real chars plus a trailing newline, mirroring PaddleOCR dicts.
	if err := os.WriteFile(p, []byte("a\nb\nc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cs, err := loadDict(p)
	if err != nil {
		t.Fatal(err)
	}
	// blank + a,b,c + trailing space = 5
	want := []string{"", "a", "b", "c", " "}
	if len(cs) != len(want) {
		t.Fatalf("charset len = %d, want %d (%q)", len(cs), len(want), cs)
	}
	for i := range want {
		if cs[i] != want[i] {
			t.Errorf("charset[%d] = %q, want %q", i, cs[i], want[i])
		}
	}
}

func TestCTCDecode(t *testing.T) {
	charset := []string{"", "a", "b", " "}
	classes := len(charset)
	// Timesteps: a, a (repeat -> collapse), blank, b  => "ab"
	rows := [][]float32{
		{0.1, 0.8, 0.05, 0.05},  // a
		{0.1, 0.7, 0.1, 0.1},    // a (repeat)
		{0.9, 0.05, 0.03, 0.02}, // blank
		{0.1, 0.1, 0.7, 0.1},    // b
	}
	var data []float32
	for _, r := range rows {
		data = append(data, r...)
	}
	text, conf := ctcDecode(data, len(rows), classes, charset)
	if text != "ab" {
		t.Errorf("text = %q, want %q", text, "ab")
	}
	if conf <= 0 || conf > 1 {
		t.Errorf("conf = %v, want (0,1]", conf)
	}
}

func TestRoundTo32(t *testing.T) {
	cases := map[float64]int{0: 32, 10: 32, 16: 32, 48: 64, 47: 32, 960: 960, 970: 960}
	for in, want := range cases {
		if got := roundTo32(in); got != want {
			t.Errorf("roundTo32(%v) = %d, want %d", in, got, want)
		}
	}
}

func TestExtractBoxes(t *testing.T) {
	// 20x20 prob map with a single high-probability 6x4 blob.
	ow, oh := 20, 20
	prob := make([]float32, ow*oh)
	for y := 5; y < 9; y++ {
		for x := 4; x < 10; x++ {
			prob[y*ow+x] = 0.9
		}
	}
	boxes := extractBoxes(prob, oh, ow, ow, oh, 1, 1)
	if len(boxes) != 1 {
		t.Fatalf("got %d boxes, want 1", len(boxes))
	}
	b := boxes[0]
	// Unclip expands the box, so it should contain the blob extent.
	if b.Min.X > 4 || b.Min.Y > 5 || b.Max.X < 10 || b.Max.Y < 9 {
		t.Errorf("box %v does not cover blob [4,5]-[10,9]", b)
	}
}

// TestRecognizeIntegration exercises the full det+rec pipeline against the baked
// models. Skips when the model files are absent (e.g. macOS dev box, CI without
// `task download-ocr-model`), so it only runs where the Docker assets exist.
func TestRecognizeIntegration(t *testing.T) {
	det := "../../models/ppocr-det.onnx"
	rec := "../../models/ppocr-rec.onnx"
	dict := "../../models/ppocr-dict.txt"
	for _, p := range []string{det, rec, dict} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("ocr models not present (%s); run `task download-ocr-model`", p)
		}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	eng, err := newEngine(Options{DetModelPath: det, RecModelPath: rec, DictPath: dict}, logger)
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}
	defer eng.(*onnxEngine).Destroy()

	img := renderText(t, "HELLO WORLD")
	blocks, err := eng.Recognize(context.Background(), img, "")
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("no text blocks recognized")
	}
	var all strings.Builder
	for _, b := range blocks {
		all.WriteString(strings.ToUpper(b.Text))
		all.WriteString(" ")
	}
	got := all.String()
	t.Logf("recognized: %q", got)
	// Loose assertion: pipeline wired correctly should recover most letters.
	if !strings.Contains(got, "HELLO") && !strings.Contains(got, "WORLD") {
		t.Errorf("recognized text %q contains neither HELLO nor WORLD", got)
	}
}

// renderText draws black text on a white canvas and scales it up so PP-OCR has
// enough resolution to read it.
func renderText(t *testing.T, s string) image.Image {
	t.Helper()
	base := image.NewRGBA(image.Rect(0, 0, 160, 30))
	for i := range base.Pix {
		base.Pix[i] = 0xff // white
	}
	d := &font.Drawer{
		Dst:  base,
		Src:  image.NewUniform(color.Black),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(8, 20),
	}
	d.DrawString(s)
	return imaging.Resize(base, 160*4, 30*4, imaging.Lanczos)
}
