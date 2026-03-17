package thumbnail

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// createTestImage creates a simple solid-color PNG test image.
func createTestImage(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating test image: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding test image: %v", err)
	}
}

func TestGenerateImageThumbnail_PNG(t *testing.T) {
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "test.png")
	createTestImage(t, srcPath, 640, 480)

	dstDir := t.TempDir()
	dstPath := filepath.Join(dstDir, "thumb.jpg")

	if err := GenerateImageThumbnail(srcPath, dstPath); err != nil {
		t.Fatalf("GenerateImageThumbnail failed: %v", err)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("thumbnail not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("thumbnail is empty")
	}
}

func TestGenerateImageThumbnail_WebP(t *testing.T) {
	// Use cwebp to create a WebP test image from a PNG
	if _, err := exec.LookPath("cwebp"); err != nil {
		t.Skip("cwebp not available")
	}

	srcDir := t.TempDir()
	pngPath := filepath.Join(srcDir, "test.png")
	createTestImage(t, pngPath, 640, 480)

	webpPath := filepath.Join(srcDir, "test.webp")
	cmd := exec.Command("cwebp", pngPath, "-o", webpPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("creating WebP test image: %v\n%s", err, output)
	}

	dstDir := t.TempDir()
	dstPath := filepath.Join(dstDir, "thumb.jpg")

	if err := GenerateImageThumbnail(webpPath, dstPath); err != nil {
		t.Fatalf("GenerateImageThumbnail failed for WebP: %v", err)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("thumbnail not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("thumbnail is empty")
	}
}
