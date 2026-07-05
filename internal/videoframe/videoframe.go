package videoframe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// extractConcurrency bounds parallel ffmpeg frame extractions per video.
const extractConcurrency = 4

// ffmpegSem bounds ffmpeg frame extractions process-wide. Extractions come
// from thumbnail tiles, analysis scans, and single-file handlers at once;
// without a global bound N concurrent requests spawn N×4 ffmpeg processes.
var ffmpegSem = make(chan struct{}, extractConcurrency)

// ExtractFrameAt runs ffmpeg to extract a single frame at the given position
// (in seconds) from videoPath into dstPath. Concurrency is bounded
// process-wide; on failure the destination file is removed.
func ExtractFrameAt(ctx context.Context, videoPath, dstPath string, seconds float64) error {
	select {
	case ffmpegSem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-ffmpegSem }()

	args := []string{
		"-ss", fmt.Sprintf("%.2f", seconds),
		"-i", videoPath,
		"-vframes", "1",
		"-f", "mjpeg",
		"-y",
		dstPath,
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(dstPath)
		return fmt.Errorf("ffmpeg failed: %w\noutput: %s", err, output)
	}
	return nil
}

// ExtractFrame extracts a single full-resolution frame from a video file at
// approximately 10% of its duration. The dir parameter controls where the
// temporary JPEG is written — use a dedicated app-managed directory rather
// than the OS temp dir so leftover files are discoverable and cleanable.
// The returned cleanup function returns an error if removal fails.
func ExtractFrame(ctx context.Context, dir, videoPath string) (framePath string, cleanup func() error, err error) {
	if _, lookErr := exec.LookPath("ffmpeg"); lookErr != nil {
		return "", nil, fmt.Errorf("ffmpeg not found in PATH: %w", lookErr)
	}

	if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
		return "", nil, fmt.Errorf("creating frame dir: %w", mkdirErr)
	}

	tmp, err := os.CreateTemp(dir, "videoframe-*.jpg")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}
	_ = tmp.Close()

	if err := ExtractFrameAt(ctx, videoPath, tmp.Name(), seekPositionSeconds(ctx, videoPath)); err != nil {
		return "", nil, err
	}

	cleanup = func() error { return os.Remove(tmp.Name()) }
	return tmp.Name(), cleanup, nil
}

// CleanDir removes all temporary frame files from the given directory.
// Intended for startup cleanup of leftover files from previous runs.
func CleanDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading frame dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := os.Remove(dir + "/" + e.Name()); err != nil {
			return fmt.Errorf("removing leftover frame %s: %w", e.Name(), err)
		}
	}
	return nil
}

// SeekPosition probes video duration and returns a timestamp string at ~10%.
// Falls back to "1" if probing fails.
func SeekPosition(ctx context.Context, videoPath string) string {
	duration, err := ProbeDuration(ctx, videoPath)
	if err != nil || duration <= 0 {
		return "1"
	}
	return strconv.FormatFloat(seekFraction(duration), 'f', 2, 64)
}

// seekPositionSeconds probes video duration and returns a position at ~10%,
// falling back to 1s if probing fails.
func seekPositionSeconds(ctx context.Context, videoPath string) float64 {
	duration, err := ProbeDuration(ctx, videoPath)
	if err != nil || duration <= 0 {
		return 1
	}
	return seekFraction(duration)
}

// seekFraction returns a seek position at ~10% of duration, snapping to the
// start for very short videos.
func seekFraction(duration float64) float64 {
	seek := duration * 0.1
	if seek < 1 {
		return 0
	}
	return seek
}

// EvenlySpacedPositions returns count timestamps evenly distributed across the
// given duration (in seconds). Positions land at 1/(count+1), 2/(count+1), …
// fractions of the duration. If duration <= 0 every position defaults to 1s.
func EvenlySpacedPositions(duration float64, count int) []float64 {
	positions := make([]float64, 0, count)
	if count <= 0 {
		return positions
	}
	if duration <= 0 {
		for range count {
			positions = append(positions, 1)
		}
		return positions
	}
	for i := range count {
		fraction := float64(i+1) / float64(count+1)
		positions = append(positions, duration*fraction)
	}
	return positions
}

// ExtractFrames extracts multiple full-resolution frames at evenly-spaced
// positions across the video. Returns the paths of successfully extracted
// frames and a cleanup function that removes all of them. At least one frame
// must succeed or an error is returned.
func ExtractFrames(ctx context.Context, dir, videoPath string, count int) (framePaths []string, cleanup func() error, err error) {
	if _, lookErr := exec.LookPath("ffmpeg"); lookErr != nil {
		return nil, nil, fmt.Errorf("ffmpeg not found in PATH: %w", lookErr)
	}
	if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
		return nil, nil, fmt.Errorf("creating frame dir: %w", mkdirErr)
	}

	duration, _ := ProbeDuration(ctx, videoPath)
	positions := EvenlySpacedPositions(duration, count)

	// Extract frames concurrently — each position is an independent ffmpeg
	// run. Index-addressed slots keep frame order deterministic for callers
	// that aggregate per-frame results. The limit bounds both ffmpeg
	// processes and concurrent readers of the same file on network mounts.
	slots := make([]string, len(positions))
	var g errgroup.Group
	g.SetLimit(extractConcurrency)
	for i, pos := range positions {
		g.Go(func() error {
			tmp, tmpErr := os.CreateTemp(dir, fmt.Sprintf("videoframe-%d-*.jpg", i))
			if tmpErr != nil {
				return nil
			}
			_ = tmp.Close()

			if runErr := ExtractFrameAt(ctx, videoPath, tmp.Name(), pos); runErr != nil {
				return nil
			}
			slots[i] = tmp.Name()
			return nil
		})
	}
	_ = g.Wait() // goroutines never return errors; failed frames leave empty slots

	paths := make([]string, 0, len(slots))
	for _, p := range slots {
		if p != "" {
			paths = append(paths, p)
		}
	}

	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("failed to extract any frames from %s", videoPath)
	}

	cleanup = func() error {
		var firstErr error
		for _, p := range paths {
			if rmErr := os.Remove(p); rmErr != nil && firstErr == nil {
				firstErr = rmErr
			}
		}
		return firstErr
	}
	return paths, cleanup, nil
}

// ProbeDuration uses ffprobe to get the video duration in seconds.
func ProbeDuration(ctx context.Context, videoPath string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	return strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
}
