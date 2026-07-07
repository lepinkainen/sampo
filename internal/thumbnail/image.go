package thumbnail

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register WebP decoder
)

const thumbSize = 300

// GenerateImageThumbnail creates a thumbnail for an image file. The imaging
// operations themselves are not cancellable, so ctx is only checked before
// the decode/resize work starts.
func GenerateImageThumbnail(ctx context.Context, srcPath, dstPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// imaging.Open handles JPEG, PNG, GIF, BMP, TIFF natively; WebP via golang.org/x/image/webp import
	src, err := imaging.Open(srcPath)
	if err != nil {
		return fmt.Errorf("opening image %s: %w", srcPath, err)
	}

	return GenerateImageThumbnailFromImage(ctx, src, dstPath)
}

// GenerateImageThumbnailFromImage creates a thumbnail from an already-decoded
// image. Callers that already paid the file read/decode cost can share that
// decode with thumbnail generation.
func GenerateImageThumbnailFromImage(ctx context.Context, src image.Image, dstPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if src == nil {
		return fmt.Errorf("nil source image")
	}

	thumb := imaging.Fit(src, thumbSize, thumbSize, imaging.Lanczos)
	return writeJPEGAtomic(dstPath, thumb)
}

func writeJPEGAtomic(dstPath string, img image.Image) error {
	dir := filepath.Dir(dstPath)
	if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
		return fmt.Errorf("creating thumbnail dir: %w", mkdirErr)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(dstPath)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary thumbnail file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	encodeErr := jpeg.Encode(tmp, img, &jpeg.Options{Quality: 80})
	closeErr := tmp.Close()

	if encodeErr != nil {
		return fmt.Errorf("encoding thumbnail: %w", encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing thumbnail file: %w", closeErr)
	}
	if err := os.Rename(tmpPath, dstPath); err != nil {
		return fmt.Errorf("moving thumbnail into place: %w", err)
	}

	return nil
}
