package thumbnail

import (
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register WebP decoder
)

const thumbSize = 300

// GenerateImageThumbnail creates a thumbnail for an image file.
func GenerateImageThumbnail(srcPath, dstPath string) error {
	// imaging.Open handles JPEG, PNG, GIF, BMP, TIFF natively; WebP via golang.org/x/image/webp import
	src, err := imaging.Open(srcPath)
	if err != nil {
		return fmt.Errorf("opening image %s: %w", srcPath, err)
	}

	thumb := imaging.Fit(src, thumbSize, thumbSize, imaging.Lanczos)

	if mkdirErr := os.MkdirAll(filepath.Dir(dstPath), 0o755); mkdirErr != nil {
		return fmt.Errorf("creating thumbnail dir: %w", mkdirErr)
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("creating thumbnail file: %w", err)
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
