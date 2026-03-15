package thumbnail

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GenerateVideoThumbnail creates a thumbnail from a video file using ffmpeg.
// It extracts a frame at approximately 10% of the video duration.
func GenerateVideoThumbnail(srcPath, dstPath string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("creating thumbnail dir: %w", err)
	}

	seekPos := getSeekPosition(srcPath)

	args := []string{
		"-ss", seekPos,
		"-i", srcPath,
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", thumbSize, thumbSize),
		"-f", "mjpeg",
		"-y",
		dstPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\noutput: %s", err, output)
	}

	return nil
}

// getSeekPosition probes video duration and returns a timestamp at ~10%.
// Falls back to "1" if probing fails.
func getSeekPosition(srcPath string) string {
	duration, err := probeDuration(srcPath)
	if err != nil || duration <= 0 {
		return "1"
	}

	seek := duration * 0.1
	if seek < 1 {
		seek = 0
	}
	return strconv.FormatFloat(seek, 'f', 2, 64)
}

// probeDuration uses ffprobe to get the video duration in seconds.
func probeDuration(srcPath string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		srcPath,
	}

	cmd := exec.Command("ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	return strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
}
