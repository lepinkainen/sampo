package thumbnail

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrune_RemovesOldFiles(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory (simulating a rootID)
	rootDir := filepath.Join(dir, "root-0")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(rootDir, "old.jpg")
	newFile := filepath.Join(rootDir, "new.jpg")
	if err := os.WriteFile(oldFile, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Backdate the old file by 100 days
	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	removed, err := cache.Prune(90 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("Prune returned error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file should have been removed")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("new file should still exist")
	}
}

func TestPrune_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	removed, err := cache.Prune(24 * time.Hour)
	if err != nil {
		t.Fatalf("Prune returned error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
}

func TestPrune_ZeroMaxAge(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	rootDir := filepath.Join(dir, "root-0")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		if err := os.WriteFile(filepath.Join(rootDir, name), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := cache.Prune(0)
	if err != nil {
		t.Fatalf("Prune returned error: %v", err)
	}
	if removed != 3 {
		t.Fatalf("expected 3 removed, got %d", removed)
	}
}

func TestPrune_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(dir, "root-0", "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	removed, err := cache.Prune(0)
	if err != nil {
		t.Fatalf("Prune returned error: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}

	if _, err := os.Stat(subDir); err != nil {
		t.Error("subdirectory should still exist")
	}
}
