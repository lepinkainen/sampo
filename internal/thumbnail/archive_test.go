package thumbnail

import (
	"archive/zip"
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/lepinkainen/sampo/internal/archive"
)

// writeCbz creates a .cbz (zip) file at path containing the given
// name->content entries.
func writeCbz(t *testing.T, path string, contents map[string][]byte, order []string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating cbz file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for _, name := range order {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("creating zip entry %s: %v", name, err)
		}
		if _, err := w.Write(contents[name]); err != nil {
			t.Fatalf("writing zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}
}

// encodePNG returns tiny solid-color PNG bytes, reusing the same approach
// as createTestImage in image_test.go but returning bytes instead of
// writing a file directly.
func encodePNGBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "src.png")
	createTestImage(t, path, width, height)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading generated PNG: %v", err)
	}
	return data
}

func TestGenerateArchiveThumbnail_CBZ(t *testing.T) {
	dir := t.TempDir()
	cbzPath := filepath.Join(dir, "comic.cbz")

	cover := encodePNGBytes(t, 640, 480)
	writeCbz(t, cbzPath, map[string][]byte{
		"01.png": cover,
	}, []string{"01.png"})

	dstPath := filepath.Join(t.TempDir(), "thumb.jpg")
	if err := GenerateArchiveThumbnail(context.Background(), cbzPath, dstPath); err != nil {
		t.Fatalf("GenerateArchiveThumbnail failed: %v", err)
	}

	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("thumbnail not created: %v", err)
	}
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Fatalf("thumbnail is not a valid JPEG, got % x", data[:min(len(data), 4)])
	}

	f, err := os.Open(dstPath)
	if err != nil {
		t.Fatalf("reopening thumbnail: %v", err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decoding thumbnail config: %v", err)
	}
	if cfg.Width > thumbSize || cfg.Height > thumbSize {
		t.Errorf("thumbnail dimensions %dx%d exceed max %d", cfg.Width, cfg.Height, thumbSize)
	}
}

func TestGenerateArchiveThumbnail_NoImages(t *testing.T) {
	dir := t.TempDir()
	cbzPath := filepath.Join(dir, "textonly.cbz")

	writeCbz(t, cbzPath, map[string][]byte{
		"readme.txt": []byte("hello"),
	}, []string{"readme.txt"})

	dstPath := filepath.Join(t.TempDir(), "thumb.jpg")
	err := GenerateArchiveThumbnail(context.Background(), cbzPath, dstPath)
	if !errors.Is(err, archive.ErrNoImages) {
		t.Fatalf("expected archive.ErrNoImages, got %v", err)
	}
}
