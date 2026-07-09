package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// pngBytes encodes a tiny solid-color PNG, mirroring the style of
// createTestImage in internal/thumbnail/image_test.go.
func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG: %v", err)
	}
	return buf.Bytes()
}

// writeZip creates a zip file at path containing the given name->content
// entries, written in the given order.
func writeZip(t *testing.T, path string, order []string, contents map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating zip file: %v", err)
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

func TestReadCoverImage_Zip_SortedNameWinsOverWriteOrder(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "comic.cbz")

	img01 := pngBytes(t, 4, 4)
	img02 := pngBytes(t, 4, 4)

	// Written in reverse order: 02 then 01.
	writeZip(t, zipPath, []string{"02.png", "01.png"}, map[string][]byte{
		"02.png": img02,
		"01.png": img01,
	})

	data, name, err := ReadCoverImage(zipPath)
	if err != nil {
		t.Fatalf("ReadCoverImage failed: %v", err)
	}
	if name != "01.png" {
		t.Errorf("expected cover 01.png (sorted order), got %q", name)
	}
	if len(data) != len(img01) {
		t.Errorf("expected data matching 01.png (%d bytes), got %d bytes", len(img01), len(data))
	}
}

func TestReadCoverImage_Zip_FiltersNonCandidates(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "comic.cbz")

	cover := pngBytes(t, 4, 4)

	order := []string{"readme.txt", "__MACOSX/._foo.png", ".hidden.png", "nested/sub.zip", "cover.png"}
	contents := map[string][]byte{
		"readme.txt":         []byte("hello"),
		"__MACOSX/._foo.png": pngBytes(t, 2, 2),
		".hidden.png":        pngBytes(t, 2, 2),
		"nested/sub.zip":     []byte("not really a zip"),
		"cover.png":          cover,
	}
	writeZip(t, zipPath, order, contents)

	data, name, err := ReadCoverImage(zipPath)
	if err != nil {
		t.Fatalf("ReadCoverImage failed: %v", err)
	}
	if name != "cover.png" {
		t.Errorf("expected cover.png (only real candidate), got %q", name)
	}
	if len(data) != len(cover) {
		t.Errorf("expected data matching cover.png (%d bytes), got %d bytes", len(cover), len(data))
	}
}

func TestReadCoverImage_Zip_NoImages(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "empty.cbz")

	writeZip(t, zipPath, []string{"readme.txt"}, map[string][]byte{
		"readme.txt": []byte("hello"),
	})

	_, _, err := ReadCoverImage(zipPath)
	if !errors.Is(err, ErrNoImages) {
		t.Fatalf("expected ErrNoImages, got %v", err)
	}
}

func TestReadCoverImage_Zip_OversizedEntrySkippedInFavorOfNext(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "comic.cbz")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("creating zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	// "01.png" declares an uncompressed size over the cap via a raw entry
	// header. readZipCover rejects it from the header alone (it is never
	// opened), so the raw payload can be tiny junk — no 64MB fixture needed.
	rawJunk := []byte("not real deflate data, never read")
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               "01.png",
		Method:             zip.Deflate,
		CompressedSize64:   uint64(len(rawJunk)),
		UncompressedSize64: maxCoverBytes + 1,
	})
	if err != nil {
		t.Fatalf("creating raw oversized entry: %v", err)
	}
	if _, err := w.Write(rawJunk); err != nil {
		t.Fatalf("writing raw oversized entry: %v", err)
	}

	// "02.png" is a normal small image that should win after the skip.
	img02 := pngBytes(t, 4, 4)
	w2, err := zw.Create("02.png")
	if err != nil {
		t.Fatalf("creating zip entry 02.png: %v", err)
	}
	if _, err := w2.Write(img02); err != nil {
		t.Fatalf("writing zip entry 02.png: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}

	data, name, err := ReadCoverImage(zipPath)
	if err != nil {
		t.Fatalf("ReadCoverImage failed: %v", err)
	}
	if name != "02.png" {
		t.Errorf("expected oversized 01.png to be skipped in favor of 02.png, got %q", name)
	}
	if len(data) != len(img02) {
		t.Errorf("expected data matching 02.png (%d bytes), got %d bytes", len(img02), len(data))
	}
}

func TestReadCoverImage_Zip_GarbageBytes(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "garbage.zip")
	if err := os.WriteFile(zipPath, []byte("this is not a zip file at all"), 0o644); err != nil {
		t.Fatalf("writing garbage file: %v", err)
	}

	_, _, err := ReadCoverImage(zipPath)
	if err == nil {
		t.Fatal("expected an error for garbage zip bytes, got nil")
	}
}

func TestSelectCoverName(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"empty", nil, ""},
		{"unsorted returns first lexicographically", []string{"b.png", "a.png", "c.png"}, "a.png"},
		{"single", []string{"only.png"}, "only.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectCoverName(tt.input); got != tt.want {
				t.Errorf("selectCoverName(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- RAR fixture-dependent tests ---
//
// These reference binary fixtures that do not exist yet, because there is
// no `rar` CLI available in this environment to create them. Each test
// fails clearly (not skips) with instructions for creating the fixture.

func TestReadCoverImage_Rar_Cover(t *testing.T) {
	fixturePath := filepath.Join("testdata", "cover.cbr")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Fatalf("missing testdata fixture %s — create with the rar CLI: "+
			"a RAR archive (renamed/saved as .cbr) containing exactly two PNG "+
			"entries added in this order: \"01.png\" then \"02.png\" (each a "+
			"small, e.g. 4x4 or 8x8, solid-color PNG, well under 1KB each). "+
			"ReadCoverImage on a rar/cbr picks the FIRST image in stream order "+
			"(no cross-entry sort, unlike zip), so with 01.png added first the "+
			"expected cover is \"01.png\".", fixturePath)
	}

	data, name, err := ReadCoverImage(fixturePath)
	if err != nil {
		t.Fatalf("ReadCoverImage failed: %v", err)
	}
	if name != "01.png" {
		t.Errorf("expected first-in-stream-order cover 01.png, got %q", name)
	}
	if len(data) == 0 {
		t.Error("expected non-empty cover data")
	}
}

func TestReadCoverImage_Rar_Encrypted(t *testing.T) {
	fixturePath := filepath.Join("testdata", "encrypted.rar")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Fatalf("missing testdata fixture %s — create with the rar CLI: "+
			"a RAR archive with at least one image entry (e.g. \"01.png\"), "+
			"created with a password (header and/or file encryption enabled, "+
			"e.g. `rar a -hp<password> encrypted.rar 01.png` for full header "+
			"encryption). ReadCoverImage must return an error satisfying "+
			"errors.Is(err, archive.ErrEncrypted).", fixturePath)
	}

	_, _, err := ReadCoverImage(fixturePath)
	if !errors.Is(err, ErrEncrypted) {
		t.Fatalf("expected ErrEncrypted, got %v", err)
	}
}

func TestReadCoverImage_Rar_Corrupt(t *testing.T) {
	dir := t.TempDir()
	rarPath := filepath.Join(dir, "corrupt.rar")
	if err := os.WriteFile(rarPath, []byte("this is not a rar file at all, just garbage bytes"), 0o644); err != nil {
		t.Fatalf("writing garbage file: %v", err)
	}

	_, _, err := ReadCoverImage(rarPath)
	if err == nil {
		t.Fatal("expected an error for garbage rar bytes, got nil")
	}
	if errors.Is(err, ErrNoImages) {
		t.Error("garbage bytes should not be reported as ErrNoImages")
	}
	if errors.Is(err, ErrEncrypted) {
		t.Error("garbage bytes should not be reported as ErrEncrypted")
	}
}
