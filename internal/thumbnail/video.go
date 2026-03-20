package thumbnail

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lepinkainen/filemanager/internal/videoframe"
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

	seekPos := videoframe.SeekPosition(srcPath)

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
