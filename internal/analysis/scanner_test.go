package analysis

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/lepinkainen/sampo/internal/classification"
	"github.com/lepinkainen/sampo/internal/config"
	"github.com/lepinkainen/sampo/internal/filesystem"
)

// setupPruneTest builds a scanner whose coordinator has a classification store
// but no analyzers, plus a root with one real file and a DB row for a ghost.
func setupPruneTest(t *testing.T) (*Scanner, *classification.Store) {
	t.Helper()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "photos"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "photos", "real.jpg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	store, err := classification.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, relPath := range []string{"photos/real.jpg", "photos/ghost.jpg"} {
		if err := store.Put(&classification.Result{
			RootID:    "root-0",
			RelPath:   relPath,
			Mtime:     1,
			Size:      1,
			ModelVer:  "test-model",
			ScannedAt: time.Now().UTC(),
			SHA256:    "hash-" + relPath,
		}); err != nil {
			t.Fatal(err)
		}
	}

	roots, err := filesystem.NewRootManager([]config.RootConfig{
		{Name: "test", Path: dir},
	})
	if err != nil {
		t.Fatal(err)
	}

	coord := NewCoordinator(nil, nil, store, nil, nil, nil, "", 1, 1, false, slog.Default())
	return NewScanner(coord, roots, 1, slog.Default()), store
}

func waitForScan(t *testing.T, s *Scanner) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !s.Status().Running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan did not finish in time")
}

func TestScanDirectoryPrunesMissingFiles(t *testing.T) {
	s, store := setupPruneTest(t)

	if err := s.ScanDirectory("root-0", "photos", true); err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	waitForScan(t, s)

	paths, err := store.ListPaths("root-0", "photos")
	if err != nil {
		t.Fatalf("ListPaths: %v", err)
	}
	if !slices.Equal(paths, []string{"photos/real.jpg"}) {
		t.Fatalf("paths after scan = %v, want only photos/real.jpg", paths)
	}
}

func TestScanDirectoryPrunesWhenDirectoryEmpty(t *testing.T) {
	s, store := setupPruneTest(t)

	// Remove the only real file: the scan finds nothing to analyze but must
	// still prune both stale rows.
	roots, _ := s.roots.Get("root-0")
	if err := os.Remove(filepath.Join(roots.Path, "photos", "real.jpg")); err != nil {
		t.Fatal(err)
	}

	if err := s.ScanDirectory("root-0", "photos", true); err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	waitForScan(t, s)

	paths, err := store.ListPaths("root-0", "photos")
	if err != nil {
		t.Fatalf("ListPaths: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("paths after scan = %v, want none", paths)
	}
}
