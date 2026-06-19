package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"jpeg", "photo.jpg", "image"},
		{"jpeg upper", "PHOTO.JPG", "image"},
		{"png", "image.png", "image"},
		{"webp", "file.webp", "image"},
		{"mp4", "video.mp4", "video"},
		{"mkv", "movie.mkv", "video"},
		{"zip", "archive.zip", "archive"},
		{"txt", "readme.txt", "other"},
		{"no ext", "Makefile", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMediaType(tt.input)
			if got != tt.expected {
				t.Errorf("DetectMediaType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMediaTypeHasThumb(t *testing.T) {
	tests := []struct {
		mediaType string
		expected  bool
	}{
		{"image", true},
		{"video", true},
		{"pdf", true},
		{"archive", false},
		{"other", false},
	}

	for _, tt := range tests {
		t.Run(tt.mediaType, func(t *testing.T) {
			if got := MediaTypeHasThumb(tt.mediaType); got != tt.expected {
				t.Errorf("MediaTypeHasThumb(%q) = %v, want %v", tt.mediaType, got, tt.expected)
			}
		})
	}
}

func TestListDirectory(t *testing.T) {
	// Create a temp directory with some files
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "test.jpg"), []byte("fake image"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0o644)

	entries, err := ListDirectory(dir, "/")
	if err != nil {
		t.Fatalf("ListDirectory failed: %v", err)
	}

	// Should not include hidden files
	for _, e := range entries {
		if e.Name == ".hidden" {
			t.Error("ListDirectory should skip hidden files")
		}
	}

	// Should have 3 entries (subdir, test.jpg, readme.txt)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Check that subdir is detected as directory
	for _, e := range entries {
		if e.Name == "subdir" && !e.IsDir {
			t.Error("subdir should be detected as directory")
		}
		if e.Name == "test.jpg" && e.MediaType != "image" {
			t.Errorf("test.jpg should be image, got %s", e.MediaType)
		}
	}
}
