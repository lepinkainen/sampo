package thumbnail

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	if err := GenerateImageThumbnail(context.Background(), srcPath, dstPath); err != nil {
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

// solidTestImage returns a solid-color image suitable for thumbnail generation.
func solidTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	return img
}

func TestGenerateImageThumbnailFromImage_CreatesValidThumbnail(t *testing.T) {
	cache, err := NewCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dstPath := cache.Path("root-0", "testkey")

	if err := GenerateImageThumbnailFromImage(context.Background(), solidTestImage(640, 480), dstPath); err != nil {
		t.Fatalf("GenerateImageThumbnailFromImage failed: %v", err)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("thumbnail not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("thumbnail is empty")
	}

	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Fatalf("thumbnail is not a valid JPEG, got % x", data[:min(len(data), 4)])
	}

	entries, err := os.ReadDir(filepath.Dir(dstPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file after successful write: %s", e.Name())
		}
	}

	if _, ok := cache.Get("root-0", "testkey"); !ok {
		t.Error("Cache.Get did not find the thumbnail after atomic write")
	}
}

func TestGenerateImageThumbnailFromImage_NilImageReturnsError(t *testing.T) {
	dstPath := filepath.Join(t.TempDir(), "thumb.jpg")
	err := GenerateImageThumbnailFromImage(context.Background(), nil, dstPath)
	if err == nil {
		t.Fatal("expected error for nil source image")
	}
}

func TestGenerateImageThumbnailFromImage_CreatesNestedDir(t *testing.T) {
	dstPath := filepath.Join(t.TempDir(), "nested", "deep", "thumb.jpg")
	if err := GenerateImageThumbnailFromImage(context.Background(), solidTestImage(100, 100), dstPath); err != nil {
		t.Fatalf("GenerateImageThumbnailFromImage failed: %v", err)
	}
	if _, err := os.Stat(dstPath); err != nil {
		t.Fatalf("thumbnail not created in nested dir: %v", err)
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

	if err := GenerateImageThumbnail(context.Background(), webpPath, dstPath); err != nil {
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
