package handlers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lepinkainen/sampo/internal/config"
	"github.com/lepinkainen/sampo/internal/filesystem"
)

// newTestRoot creates a temp-dir root with the given files (paths relative to
// the root; parent dirs are created as needed) and returns a RootManager
// whose only root has ID "root-0".
func newTestRoot(t *testing.T, files []string) *filesystem.RootManager {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		full := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	roots, err := filesystem.NewRootManager([]config.RootConfig{{Name: "test", Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	return roots
}

func TestSelectedOrganizeFilesIncludesSubdirectoryFiles(t *testing.T) {
	roots := newTestRoot(t, []string{
		"top.jpg",
		"sub/nested.jpg",
		"sub/deeper/leaf.jpg",
	})

	got := selectedOrganizeFiles(roots, "root-0", []string{
		"top.jpg",
		"sub/nested.jpg",
		"sub/deeper/leaf.jpg",
	})

	want := [][2]string{
		{"top.jpg", "top.jpg"},
		{"sub/nested.jpg", "nested.jpg"},
		{"sub/deeper/leaf.jpg", "leaf.jpg"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSelectedOrganizeFilesSkipsInvalidPaths(t *testing.T) {
	roots := newTestRoot(t, []string{
		"a.jpg",
		"sub/b.jpg",
		"sub/.hidden",
	})

	got := selectedOrganizeFiles(roots, "root-0", []string{
		"a.jpg",           // valid
		"missing.jpg",     // nonexistent
		"sub",             // directory
		"sub/.hidden",     // dotfile
		"../escape.jpg",   // traversal attempt (cleans to escape.jpg, nonexistent)
		"",                // empty
		"sub/missing.jpg", // nonexistent in subdir
	})

	want := [][2]string{
		{"a.jpg", "a.jpg"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSelectedOrganizeFilesUnknownRootReturnsNothing(t *testing.T) {
	roots := newTestRoot(t, []string{"a.jpg"})

	got := selectedOrganizeFiles(roots, "root-99", []string{"a.jpg"})
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}

func TestCollectOrganizeFilesReturnsTopLevelFiles(t *testing.T) {
	roots := newTestRoot(t, []string{
		"a.jpg",
		"b.jpg",
		".hidden",
		"sub/nested.jpg", // creates subdir "sub", which must be skipped
	})

	dir, err := roots.ResolvePath("root-0", "/")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	got := collectOrganizeFiles(entries, "photos")

	want := [][2]string{
		{"photos/a.jpg", "a.jpg"},
		{"photos/b.jpg", "b.jpg"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
