package thumbnail

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateVideoThumbnail(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	// Generate a 2-second test video with ffmpeg
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "test.mp4")

	genArgs := []string{
		"-f", "lavfi",
		"-i", "color=c=red:s=320x240:d=2",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-y",
		srcPath,
	}
	cmd := exec.Command("ffmpeg", genArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("creating test video: %v\n%s", err, output)
	}

	dstDir := t.TempDir()
	dstPath := filepath.Join(dstDir, "thumb.jpg")

	err := GenerateVideoThumbnail(srcPath, dstPath)
	if err != nil {
		t.Fatalf("GenerateVideoThumbnail failed: %v", err)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("thumbnail not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("thumbnail is empty")
	}
}

func TestGetSeekPosition(t *testing.T) {
	// With no valid file, should fallback to "1"
	pos := getSeekPosition("/nonexistent/file.mp4")
	if pos != "1" {
		t.Errorf("expected fallback '1', got %q", pos)
	}
}
