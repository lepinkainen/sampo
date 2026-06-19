package thumbnail

import (
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGeneratePdfThumbnail(t *testing.T) {
	// Skip if no tool is available
	hasTool := false
	for _, tool := range []string{"pdftoppm", "qlmanage", "convert"} {
		if _, err := exec.LookPath(tool); err == nil {
			hasTool = true
			break
		}
	}
	if !hasTool {
		t.Skip("No PDF thumbnail generator tool available in PATH (pdftoppm, qlmanage, convert)")
	}

	// Write a minimal PDF structure
	minimalPDF := `%PDF-1.4
1 0 obj <</Type /Catalog /Pages 2 0 R>> endobj
2 0 obj <</Type /Pages /Kids [3 0 R] /Count 1>> endobj
3 0 obj <</Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources <<>> /Contents 4 0 R>> endobj
4 0 obj <</Length 0>> stream
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000056 00000 n 
0000000111 00000 n 
0000000212 00000 n 
trailer <</Size 5 /Root 1 0 R>>
startxref
263
%%EOF`

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "test.pdf")
	if err := os.WriteFile(srcPath, []byte(minimalPDF), 0644); err != nil {
		t.Fatalf("writing test PDF: %v", err)
	}

	dstDir := t.TempDir()
	dstPath := filepath.Join(dstDir, "thumb.jpg")

	if err := GeneratePdfThumbnail(srcPath, dstPath); err != nil {
		t.Fatalf("GeneratePdfThumbnail failed: %v", err)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("thumbnail not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("thumbnail is empty")
	}

	f, err := os.Open(dstPath)
	if err != nil {
		t.Fatalf("opening thumbnail: %v", err)
	}
	defer f.Close()

	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decoding thumbnail: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("invalid thumbnail bounds: %v", bounds)
	}
}
