package videoframe

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ExtractFrame extracts a single full-resolution frame from a video file at
// approximately 10% of its duration. The dir parameter controls where the
// temporary JPEG is written — use a dedicated app-managed directory rather
// than the OS temp dir so leftover files are discoverable and cleanable.
// The returned cleanup function returns an error if removal fails.
func ExtractFrame(dir, videoPath string) (framePath string, cleanup func() error, err error) {
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

	seekPos := SeekPosition(videoPath)

	args := []string{
		"-ss", seekPos,
		"-i", videoPath,
		"-vframes", "1",
		"-f", "mjpeg",
		"-y",
		tmp.Name(),
	}

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("ffmpeg failed: %w\noutput: %s", err, output)
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
func SeekPosition(videoPath string) string {
	duration, err := ProbeDuration(videoPath)
	if err != nil || duration <= 0 {
		return "1"
	}

	seek := duration * 0.1
	if seek < 1 {
		seek = 0
	}
	return strconv.FormatFloat(seek, 'f', 2, 64)
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
func ExtractFrames(dir, videoPath string, count int) (framePaths []string, cleanup func() error, err error) {
	if _, lookErr := exec.LookPath("ffmpeg"); lookErr != nil {
		return nil, nil, fmt.Errorf("ffmpeg not found in PATH: %w", lookErr)
	}
	if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
		return nil, nil, fmt.Errorf("creating frame dir: %w", mkdirErr)
	}

	duration, _ := ProbeDuration(videoPath)
	positions := EvenlySpacedPositions(duration, count)

	var paths []string
	for i, pos := range positions {
		tmp, tmpErr := os.CreateTemp(dir, fmt.Sprintf("videoframe-%d-*.jpg", i))
		if tmpErr != nil {
			continue
		}
		_ = tmp.Close()

		args := []string{
			"-ss", fmt.Sprintf("%.2f", pos),
			"-i", videoPath,
			"-vframes", "1",
			"-f", "mjpeg",
			"-y",
			tmp.Name(),
		}
		cmd := exec.Command("ffmpeg", args...)
		if _, runErr := cmd.CombinedOutput(); runErr != nil {
			_ = os.Remove(tmp.Name())
			continue
		}
		paths = append(paths, tmp.Name())
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
func ProbeDuration(videoPath string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	}

	cmd := exec.Command("ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	return strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
}
