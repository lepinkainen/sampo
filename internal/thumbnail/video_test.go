package thumbnail

import (
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lepinkainen/filemanager/internal/videoframe"
)

func TestGenerateVideoThumbnail(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "test.mp4")

	genArgs := []string{
		"-f", "lavfi", "-i", "color=c=red:s=320x240:d=1",
		"-f", "lavfi", "-i", "color=c=green:s=320x240:d=1",
		"-f", "lavfi", "-i", "color=c=blue:s=320x240:d=1",
		"-f", "lavfi", "-i", "color=c=yellow:s=320x240:d=1",
		"-filter_complex", "[0:v][1:v][2:v][3:v]concat=n=4:v=1:a=0[outv]",
		"-map", "[outv]",
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

	if err := GenerateVideoThumbnail(srcPath, dstPath); err != nil {
		t.Fatalf("GenerateVideoThumbnail failed: %v", err)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("thumbnail not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("thumbnail is empty")
	}

	f, err := os.Open(dstPath)
	if err != nil {
		t.Fatalf("opening thumbnail: %v", err)
	}
	defer f.Close()

	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decoding thumbnail: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != thumbSize || bounds.Dy() != thumbSize {
		t.Fatalf("expected thumbnail size %dx%d, got %dx%d", thumbSize, thumbSize, bounds.Dx(), bounds.Dy())
	}
}

func TestGetSeekPosition(t *testing.T) {
	// With no valid file, should fallback to "1"
	pos := videoframe.SeekPosition("/nonexistent/file.mp4")
	if pos != "1" {
		t.Errorf("expected fallback '1', got %q", pos)
	}
}
