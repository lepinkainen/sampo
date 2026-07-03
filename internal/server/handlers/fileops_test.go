package handlers_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/classification"
	"github.com/lepinkainen/sampo/internal/config"
	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/ocr"
	"github.com/lepinkainen/sampo/internal/server/handlers"
	"github.com/lepinkainen/sampo/internal/thumbnail"
)

// setupDeleteTest builds a handler with real classification + OCR stores and
// two duplicate files on disk whose checksums are already in the database.
func setupDeleteTest(t *testing.T) (*handlers.Handler, *classification.Store, *ocr.Store, string) {
	t.Helper()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "photos"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"photos/orig.jpg", "photos/copy.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("same bytes"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	roots, err := filesystem.NewRootManager([]config.RootConfig{
		{Name: "test", Path: dir},
	})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := thumbnail.NewCache(filepath.Join(dir, ".cache"))
	if err != nil {
		t.Fatal(err)
	}

	classStore, err := classification.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = classStore.Close() })
	ocrStore, err := ocr.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ocrStore.Close() })

	for _, relPath := range []string{"photos/orig.jpg", "photos/copy.jpg"} {
		if err := classStore.Put(&classification.Result{
			RootID:    "root-0",
			RelPath:   relPath,
			Mtime:     1,
			Size:      10,
			ModelVer:  "test-model",
			ScannedAt: time.Now().UTC(),
			SHA256:    "samehash",
		}); err != nil {
			t.Fatal(err)
		}
		if err := ocrStore.Put(&ocr.Result{
			RootID:    "root-0",
			RelPath:   relPath,
			Mtime:     1,
			Size:      10,
			ModelVer:  "test-model",
			ScannedAt: time.Now().UTC(),
			Text:      "some text",
		}); err != nil {
			t.Fatal(err)
		}
	}

	h := handlers.New(roots, cache, t.TempDir(), slog.Default())
	h.SetClassification(classStore, nil, nil)
	h.SetOCR(ocrStore, nil, nil)
	return h, classStore, ocrStore, dir
}

func doDelete(h *handlers.Handler, path string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Delete("/api/files/{rootID}/*", h.DeleteFile)
	req := httptest.NewRequest("DELETE", path, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestDeleteFile_PurgesAnalysisResults(t *testing.T) {
	h, classStore, ocrStore, dir := setupDeleteTest(t)

	// Sanity: the two files are seen as duplicates before deleting.
	groups, err := classStore.FindDuplicates("root-0", "photos")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || len(groups[0].Files) != 2 {
		t.Fatalf("groups before delete = %+v, want 1 group with 2 files", groups)
	}

	rr := doDelete(h, "/api/files/root-0/photos/copy.jpg")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204: %s", rr.Code, rr.Body.String())
	}

	// Gone from disk (the file view lists the filesystem, so this is what
	// the tree/grid reflect).
	if _, err := os.Stat(filepath.Join(dir, "photos/copy.jpg")); !os.IsNotExist(err) {
		t.Fatalf("file still on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "photos/orig.jpg")); err != nil {
		t.Fatalf("surviving file missing: %v", err)
	}

	// Gone from the classification DB: no longer reported as a duplicate.
	groups, err = classStore.FindDuplicates("root-0", "photos")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("groups after delete = %+v, want none", groups)
	}

	// Gone from the OCR DB; the survivor keeps its text.
	if text, _ := ocrStore.GetText("root-0", "photos/copy.jpg"); text != "" {
		t.Fatalf("ocr text after delete = %q, want empty", text)
	}
	if text, _ := ocrStore.GetText("root-0", "photos/orig.jpg"); text != "some text" {
		t.Fatalf("survivor ocr text = %q, want kept", text)
	}
}

func TestDeleteFile_RecursivePurgesSubtree(t *testing.T) {
	h, classStore, _, dir := setupDeleteTest(t)

	rr := doDelete(h, "/api/files/root-0/photos?recursive=true")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "photos")); !os.IsNotExist(err) {
		t.Fatalf("directory still on disk: %v", err)
	}

	checksums, err := classStore.GetDirChecksums("root-0", "photos")
	if err != nil {
		t.Fatal(err)
	}
	if len(checksums) != 0 {
		t.Fatalf("checksums after recursive delete = %v, want none", checksums)
	}
}
