package thumbnail

import (
	"bytes"
	"context"
	"fmt"

	"github.com/disintegration/imaging"
	"github.com/lepinkainen/sampo/internal/archive"
)

// GenerateArchiveThumbnail extracts the cover image from the archive at
// srcPath (comic-cover convention: lexicographically-first image by name
// for zip/cbz, first image in stream order for rar/cbr) and writes a
// thumbnail to dstPath. Errors from the underlying archive read (notably
// archive.ErrNoImages and archive.ErrEncrypted) are wrapped with %w so
// callers can still detect them via errors.Is.
func GenerateArchiveThumbnail(ctx context.Context, srcPath, dstPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, _, err := archive.ReadCoverImage(srcPath)
	if err != nil {
		return fmt.Errorf("reading cover image from %s: %w", srcPath, err)
	}

	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decoding cover image from %s: %w", srcPath, err)
	}

	return GenerateImageThumbnailFromImage(ctx, img, dstPath)
}
