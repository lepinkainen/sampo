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
	"time"

	"github.com/disintegration/imaging"
	"github.com/lepinkainen/sampo/internal/videoframe"
)

const videoThumbnailFrameCount = 4

// GenerateVideoThumbnail creates a 2x2 square overview thumbnail from a video.
// It samples four evenly spaced frames across the video's duration and arranges
// them from top-left to bottom-right.
func GenerateVideoThumbnail(srcPath, dstPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("creating thumbnail dir: %w", err)
	}

	duration, err := videoframe.ProbeDuration(context.Background(), srcPath)
	if err != nil || duration <= 0 {
		duration = 0
	}

	tempDir, err := os.MkdirTemp("", "sampo-video-thumb-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tileSize := thumbSize / 2
	tiles := make([]image.Image, 0, videoThumbnailFrameCount)
	positions := videoframe.EvenlySpacedPositions(duration, videoThumbnailFrameCount)

	for _, pos := range positions {
		framePath, extractErr := extractFrameWithFallback(srcPath, tempDir, duration, pos)
		if extractErr != nil {
			return extractErr
		}

		img, openErr := imaging.Open(framePath)
		if openErr != nil {
			return fmt.Errorf("opening extracted frame %s: %w", framePath, openErr)
		}

		tile := imaging.Fill(img, tileSize, tileSize, imaging.Center, imaging.Lanczos)
		tiles = append(tiles, tile)
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

func extractFrameWithFallback(srcPath, tempDir string, duration, targetSeconds float64) (string, error) {
	candidates := fallbackSeekPositions(duration, targetSeconds)
	var lastErr error

	for idx, candidate := range candidates {
		framePath := filepath.Join(tempDir, fmt.Sprintf("frame-%0.2f-%d.jpg", candidate, idx))
		if err := extractFrameAt(srcPath, framePath, candidate); err == nil {
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

func extractFrameAt(srcPath, dstPath string, seconds float64) error {
	args := []string{
		"-ss", fmt.Sprintf("%.2f", seconds),
		"-i", srcPath,
		"-vframes", "1",
		"-f", "mjpeg",
		"-y",
		dstPath,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(dstPath)
		return fmt.Errorf("ffmpeg failed: %w\noutput: %s", err, output)
	}
	return nil
}
