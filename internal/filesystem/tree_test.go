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
		{"rar", "archive.rar", "archive"},
		{"cbz", "comic.cbz", "archive"},
		{"cbr", "comic.cbr", "archive"},
		{"cbr upper", "COMIC.CBR", "archive"},
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
		{"archive", true},
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

func TestHasImageExt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"jpg", "photo.jpg", true},
		{"jpeg", "photo.jpeg", true},
		{"png", "image.png", true},
		{"webp", "image.webp", true},
		{"gif", "anim.gif", true},
		{"bmp", "image.bmp", true},
		{"tiff", "image.tiff", true},
		{"avif", "image.avif", true},
		{"upper case", "IMAGE.PNG", true},
		{"txt", "readme.txt", false},
		{"no ext", "Makefile", false},
		{"mp4", "video.mp4", false},
		{"zip", "archive.zip", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasImageExt(tt.input); got != tt.expected {
				t.Errorf("HasImageExt(%q) = %v, want %v", tt.input, got, tt.expected)
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

func TestListDirectoryOrderAndAttributes(t *testing.T) {
	dir := t.TempDir()
	// Create out-of-order to verify listing is lexical regardless.
	names := []string{"zebra.txt", "apple.jpg", "mango", "banana.mp4"}
	os.MkdirAll(filepath.Join(dir, "mango"), 0o755)
	os.WriteFile(filepath.Join(dir, "mango", "pic.jpg"), []byte("img"), 0o644)
	for _, n := range names {
		if n == "mango" {
			continue
		}
		os.WriteFile(filepath.Join(dir, n), []byte(n), 0o644)
	}

	entries, err := ListDirectory(dir, "/")
	if err != nil {
		t.Fatalf("ListDirectory failed: %v", err)
	}

	want := []string{"apple.jpg", "banana.mp4", "mango", "zebra.txt"}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(entries))
	}
	for i, n := range want {
		if entries[i].Name != n {
			t.Errorf("entry %d: got %q, want %q (lexical order)", i, entries[i].Name, n)
		}
	}

	for _, e := range entries {
		switch e.Name {
		case "zebra.txt":
			if e.Size != int64(len("zebra.txt")) {
				t.Errorf("zebra.txt size = %d", e.Size)
			}
			if e.ModTime.IsZero() {
				t.Error("zebra.txt modTime not populated")
			}
		case "mango":
			if !e.IsDir {
				t.Error("mango should be a directory")
			}
			if !e.HasThumb {
				t.Error("mango contains an image, HasThumb should be true")
			}
		}
	}
}
