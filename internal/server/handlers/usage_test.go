package handlers_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/config"
	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/server/handlers"
	"github.com/lepinkainen/sampo/internal/thumbnail"
)

// setupUsageHandler builds a fixture tree with known sizes and counts.
//
//	root/
//	  a.txt        (3 bytes)
//	  sub/b.txt    (5 bytes)
//	  sub/nested/c.txt (7 bytes)
//	  .hidden/big.txt  (excluded)
//	  .dotfile         (excluded)
func setupUsageHandler(t *testing.T) *handlers.Handler {
	t.Helper()
	dir := t.TempDir()

	for _, d := range []string{"sub/nested", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"a.txt":            "abc",
		"sub/b.txt":        "abcde",
		"sub/nested/c.txt": "abcdefg",
		".hidden/big.txt":  "should not count",
		".dotfile":         "nope",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	roots, err := filesystem.NewRootManager([]config.RootConfig{{Name: "test", Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := thumbnail.NewCache(filepath.Join(dir, ".cache"))
	if err != nil {
		t.Fatal(err)
	}
	return handlers.New(roots, cache, t.TempDir(), slog.Default())
}

func getUsage(t *testing.T, h *handlers.Handler, path string) (int, handlers.DiskUsage) {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/api/usage/{rootID}/*", h.GetDiskUsage)

	req := httptest.NewRequest("GET", path, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	var usage handlers.DiskUsage
	if rr.Code == http.StatusOK {
		if err := json.NewDecoder(rr.Body).Decode(&usage); err != nil {
			t.Fatal(err)
		}
	}
	return rr.Code, usage
}

func TestGetDiskUsage_CountsAndSizes(t *testing.T) {
	h := setupUsageHandler(t)

	code, usage := getUsage(t, h, "/api/usage/root-0/")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if usage.TotalSize != 15 {
		t.Errorf("TotalSize = %d, want 15", usage.TotalSize)
	}
	if usage.FileCount != 3 {
		t.Errorf("FileCount = %d, want 3", usage.FileCount)
	}
	if usage.DirCount != 2 {
		t.Errorf("DirCount = %d, want 2 (sub, sub/nested; hidden excluded)", usage.DirCount)
	}
}

func TestGetDiskUsage_Subdirectory(t *testing.T) {
	h := setupUsageHandler(t)

	code, usage := getUsage(t, h, "/api/usage/root-0/sub")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if usage.TotalSize != 12 {
		t.Errorf("TotalSize = %d, want 12", usage.TotalSize)
	}
	if usage.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", usage.FileCount)
	}
	if usage.DirCount != 1 {
		t.Errorf("DirCount = %d, want 1", usage.DirCount)
	}
}

func TestGetDiskUsage_NotFound(t *testing.T) {
	h := setupUsageHandler(t)

	code, _ := getUsage(t, h, "/api/usage/root-0/missing")
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestSearchFiles_SortedAndFiltered(t *testing.T) {
	h := setupUsageHandler(t)

	r := chi.NewRouter()
	r.Get("/api/search/{rootID}", h.SearchFiles)

	req := httptest.NewRequest("GET", "/api/search/root-0?q=.txt", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var results []filesystem.FileEntry
	if err := json.NewDecoder(rr.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3 (hidden excluded): %+v", len(results), results)
	}
	if !sort.SliceIsSorted(results, func(i, j int) bool { return results[i].Path < results[j].Path }) {
		t.Errorf("results not sorted by path: %+v", results)
	}
	for _, e := range results {
		if filepath.Base(e.Path) == "big.txt" {
			t.Errorf("hidden file leaked into results: %+v", e)
		}
	}
}
