package thumbnail

import (
	"context"
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

// GeneratePdfThumbnail creates a thumbnail for the first page of a PDF file.
func GeneratePdfThumbnail(srcPath, dstPath string) error {
	// Create a temp directory for the extracted page image
	tempDir, err := os.MkdirTemp("", "sampo-pdf-thumb-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try pdftoppm first (standard in Linux/Docker poppler-utils)
	if _, errLook := exec.LookPath("pdftoppm"); errLook == nil {
		args := []string{
			"-f", "1",
			"-l", "1",
			"-png",
			"-r", "150",
			srcPath,
			filepath.Join(tempDir, "page"),
		}
		cmd = exec.CommandContext(ctx, "pdftoppm", args...)
	} else if _, errLook := exec.LookPath("qlmanage"); errLook == nil {
		// Fallback to macOS qlmanage if available
		args := []string{
			"-t",
			"-s", fmt.Sprintf("%d", thumbSize),
			"-o", tempDir,
			srcPath,
		}
		cmd = exec.CommandContext(ctx, "qlmanage", args...)
	} else if _, errLook := exec.LookPath("convert"); errLook == nil {
		// Fallback to ImageMagick convert if available
		args := []string{
			srcPath + "[0]",
			filepath.Join(tempDir, "page.png"),
		}
		cmd = exec.CommandContext(ctx, "convert", args...)
	} else {
		return fmt.Errorf("no PDF thumbnail generator found (install poppler-utils or ImageMagick)")
	}

	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return fmt.Errorf("running PDF thumbnail generator %s failed: %w (output: %s)", cmd.Path, runErr, string(output))
	}

	// Find the generated image file in tempDir
	imgPath, findErr := findFirstImageFile(tempDir)
	if findErr != nil {
		return fmt.Errorf("locating generated PDF page image: %w", findErr)
	}

	// Open, resize, and save using imaging (exactly like GenerateImageThumbnail)
	src, openErr := imaging.Open(imgPath)
	if openErr != nil {
		return fmt.Errorf("opening generated PDF page image %s: %w", imgPath, openErr)
	}

	thumb := imaging.Fit(src, thumbSize, thumbSize, imaging.Lanczos)

	if mkdirErr := os.MkdirAll(filepath.Dir(dstPath), 0o755); mkdirErr != nil {
		return fmt.Errorf("creating thumbnail dir: %w", mkdirErr)
	}

	out, createErr := os.Create(dstPath)
	if createErr != nil {
		return fmt.Errorf("creating thumbnail file: %w", createErr)
	}

	encodeErr := jpeg.Encode(out, thumb, &jpeg.Options{Quality: 80})
	closeErr := out.Close()

	if encodeErr != nil {
		return fmt.Errorf("encoding thumbnail: %w", encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing thumbnail file: %w", closeErr)
	}

	return nil
}

func findFirstImageFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no image file found in temp dir")
}
