package thumbnail

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/disintegration/imaging"
	"github.com/lepinkainen/sampo/internal/videoframe"
	"golang.org/x/sync/errgroup"
)

const videoThumbnailFrameCount = 4

// GenerateVideoThumbnail creates a 2x2 square overview thumbnail from a video.
// It samples four evenly spaced frames across the video's duration and arranges
// them from top-left to bottom-right. Cancelling ctx aborts in-flight ffmpeg runs.
func GenerateVideoThumbnail(ctx context.Context, srcPath, dstPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("creating thumbnail dir: %w", err)
	}

	duration, err := videoframe.ProbeDuration(ctx, srcPath)
	if err != nil || duration <= 0 {
		duration = 0
	}

	tempDir, err := os.MkdirTemp("", "sampo-video-thumb-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tileSize := thumbSize / 2
	positions := videoframe.EvenlySpacedPositions(duration, videoThumbnailFrameCount)
	tiles := make([]image.Image, len(positions))
	tileErrs := make([]error, len(positions))

	// Extract and process the four tiles concurrently; each is an
	// independent ffmpeg run plus decode/resize. Index addressing keeps
	// tile placement deterministic (top-left → bottom-right). ffmpeg
	// concurrency is bounded process-wide inside videoframe.ExtractFrameAt.
	var g errgroup.Group
	for i, pos := range positions {
		g.Go(func() error {
			framePath, extractErr := extractFrameWithFallback(ctx, srcPath, tempDir, i, duration, pos)
			if extractErr != nil {
				tileErrs[i] = extractErr
				return nil
			}

			img, openErr := imaging.Open(framePath)
			if openErr != nil {
				tileErrs[i] = fmt.Errorf("opening extracted frame %s: %w", framePath, openErr)
				return nil
			}

			tiles[i] = imaging.Fill(img, tileSize, tileSize, imaging.Center, imaging.Lanczos)
			return nil
		})
	}
	_ = g.Wait() // errors collected per tile; first by index returned below

	for _, tileErr := range tileErrs {
		if tileErr != nil {
			return tileErr
		}
	}

	canvas := image.NewRGBA(image.Rect(0, 0, thumbSize, thumbSize))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)

	for i, tile := range tiles {
		x := (i % 2) * tileSize
		y := (i / 2) * tileSize
		rect := image.Rect(x, y, x+tileSize, y+tileSize)
		draw.Draw(canvas, rect, tile, tile.Bounds().Min, draw.Src)
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("creating thumbnail file: %w", err)
	}

	encodeErr := jpeg.Encode(out, canvas, &jpeg.Options{Quality: 80})
	closeErr := out.Close()
	if encodeErr != nil {
		return fmt.Errorf("encoding thumbnail: %w", encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing thumbnail file: %w", closeErr)
	}

	return nil
}

// extractFrameWithFallback tries candidate seek positions near targetSeconds
// until one yields a frame. tile disambiguates temp filenames — concurrent
// tiles on short videos can share candidate timestamps.
func extractFrameWithFallback(ctx context.Context, srcPath, tempDir string, tile int, duration, targetSeconds float64) (string, error) {
	candidates := fallbackSeekPositions(duration, targetSeconds)
	var lastErr error

	for idx, candidate := range candidates {
		framePath := filepath.Join(tempDir, fmt.Sprintf("frame-%d-%0.2f-%d.jpg", tile, candidate, idx))
		if err := videoframe.ExtractFrameAt(ctx, srcPath, framePath, candidate); err == nil {
			return framePath, nil
		} else {
			lastErr = err
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate timestamps available")
	}
	return "", fmt.Errorf("extracting frame near %.2fs: %w", targetSeconds, lastErr)
}

func fallbackSeekPositions(duration, target float64) []float64 {
	if duration <= 0 {
		return []float64{1, 0}
	}

	clamp := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > duration {
			return duration
		}
		return v
	}

	offsets := []float64{
		0,
		duration * 0.02, -duration * 0.02,
		duration * 0.05, -duration * 0.05,
		duration * 0.1, -duration * 0.1,
		1, -1,
		2, -2,
	}

	seen := make(map[int64]bool, len(offsets))
	positions := make([]float64, 0, len(offsets))
	for _, offset := range offsets {
		pos := clamp(target + offset)
		key := int64(math.Round(pos * 100))
		if seen[key] {
			continue
		}
		seen[key] = true
		positions = append(positions, pos)
	}
	return positions
}
