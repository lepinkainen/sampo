package handlers_test

import (
	"encoding/json"
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

func setupTestHandler(t *testing.T) (*handlers.Handler, string) {
	t.Helper()

	dir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
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

	h := handlers.New(roots, cache, t.TempDir(), slog.Default())
	return h, dir
}

func serveWithChi(h *handlers.Handler, method, path string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/api/file/{rootID}/*", h.ServeFile)

	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestServeFile_ValidFile(t *testing.T) {
	h, _ := setupTestHandler(t)

	rr := serveWithChi(h, "GET", "/api/file/root-0/test.txt")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "hello world" {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
}

func TestServeFile_Directory(t *testing.T) {
	h, _ := setupTestHandler(t)

	rr := serveWithChi(h, "GET", "/api/file/root-0/subdir")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for directory, got %d", rr.Code)
	}
}

func TestServeFile_NotFound(t *testing.T) {
	h, _ := setupTestHandler(t)

	rr := serveWithChi(h, "GET", "/api/file/root-0/nonexistent.txt")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestServeFile_InvalidRoot(t *testing.T) {
	h, _ := setupTestHandler(t)

	rr := serveWithChi(h, "GET", "/api/file/bad-root/test.txt")

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid root, got %d", rr.Code)
	}
}

func TestServeFile_MKVContentType(t *testing.T) {
	h, dir := setupTestHandler(t)

	mkvPath := filepath.Join(dir, "sample.mkv")
	if err := os.WriteFile(mkvPath, []byte("not-a-real-video"), 0644); err != nil {
		t.Fatal(err)
	}

	rr := serveWithChi(h, "GET", "/api/file/root-0/sample.mkv")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "video/x-matroska" {
		t.Errorf("expected Content-Type video/x-matroska, got %q", got)
	}
}

// setupSpecialCharHandler creates a test handler with files/dirs containing special characters.
func setupSpecialCharHandler(t *testing.T) *handlers.Handler {
	t.Helper()

	dir := t.TempDir()

	// Directory with ampersand
	if err := os.MkdirAll(filepath.Join(dir, "dir&name"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dir&name", "test.txt"), []byte("ampersand"), 0644); err != nil {
		t.Fatal(err)
	}

	// Directory with apostrophe
	if err := os.MkdirAll(filepath.Join(dir, "dir'name"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dir'name", "test.txt"), []byte("apostrophe-dir"), 0644); err != nil {
		t.Fatal(err)
	}

	// File with apostrophe
	if err := os.WriteFile(filepath.Join(dir, "file'name.txt"), []byte("apostrophe"), 0644); err != nil {
		t.Fatal(err)
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

	return handlers.New(roots, cache, t.TempDir(), slog.Default())
}

// serveAllWithChi registers all API routes and serves a request.
func serveAllWithChi(h *handlers.Handler, method, path string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/api/file/{rootID}/*", h.ServeFile)
	r.Get("/api/tree/{rootID}/*", h.ListDirectory)
	r.Get("/api/thumb/{rootID}/*", h.GetThumbnail)

	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestListDirectory_ClassificationTagsReflectReplacement(t *testing.T) {
	h, dir := setupTestHandler(t)

	if err := os.WriteFile(filepath.Join(dir, "subdir", "tagged.jpg"), []byte("fake image"), 0644); err != nil {
		t.Fatal(err)
	}

	store, err := classification.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	h.SetClassification(store, nil, nil)

	putHandlerClassificationResult(t, store, "subdir/tagged.jpg", []classification.TagScore{
		{Label: "legacy", Score: 0.9},
		{Label: "stale", Score: 0.8},
	})
	putHandlerClassificationResult(t, store, "subdir/tagged.jpg", []classification.TagScore{
		{Label: "fresh", Score: 0.95},
	})

	rr := serveAllWithChi(h, "GET", "/api/tree/root-0/subdir")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	entries := decodeFileEntries(t, rr)
	tags := tagsFor(entries, "subdir/tagged.jpg")
	if len(tags) != 1 || tags[0].Label != "fresh" {
		t.Fatalf("tags = %#v, want only fresh", tags)
	}
}

func TestListDirectory_OCRTextUsesCanonicalRelPath(t *testing.T) {
	h, dir := setupTestHandler(t)

	cacheDir := filepath.Join(dir, ".ocr-cache")
	if err := os.Mkdir(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	store, err := ocr.NewStore(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	h.SetOCR(store, nil, nil)

	if err := os.WriteFile(filepath.Join(dir, "subdir", "test.txt"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	putHandlerOCRResult(t, store, "test.txt", "root text")
	putHandlerOCRResult(t, store, "subdir/test.txt", "nested text")

	rr := serveAllWithChi(h, "GET", "/api/tree/root-0/")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	entries := decodeFileEntries(t, rr)
	if got := ocrTextFor(entries, "/test.txt"); got != "root text" {
		t.Fatalf("root OCR text = %q, want root text (entries %#v)", got, entries)
	}

	rr = serveAllWithChi(h, "GET", "/api/tree/root-0/subdir")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	entries = decodeFileEntries(t, rr)
	if got := ocrTextFor(entries, "subdir/test.txt"); got != "nested text" {
		t.Fatalf("nested OCR text = %q, want nested text (entries %#v)", got, entries)
	}
}

func putHandlerClassificationResult(t *testing.T, store *classification.Store, relPath string, tags []classification.TagScore) {
	t.Helper()
	if err := store.Put(&classification.Result{
		RootID:    "root-0",
		RelPath:   relPath,
		Mtime:     1,
		Size:      1,
		ModelVer:  "test-model",
		ScannedAt: time.Now().UTC(),
		Tags:      tags,
	}); err != nil {
		t.Fatalf("Put classification result: %v", err)
	}
}

func putHandlerOCRResult(t *testing.T, store *ocr.Store, relPath, text string) {
	t.Helper()
	if err := store.Put(&ocr.Result{
		RootID:    "root-0",
		RelPath:   relPath,
		Mtime:     1,
		Size:      1,
		ModelVer:  "test-model",
		ScannedAt: time.Now().UTC(),
		Text:      text,
		Blocks:    []ocr.TextBlock{{Text: text}},
	}); err != nil {
		t.Fatalf("Put OCR result: %v", err)
	}
}

func decodeFileEntries(t *testing.T, rr *httptest.ResponseRecorder) []filesystem.FileEntry {
	t.Helper()
	var entries []filesystem.FileEntry
	if err := json.Unmarshal(rr.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode entries: %v", err)
	}
	return entries
}

func tagsFor(entries []filesystem.FileEntry, relPath string) []filesystem.TagScore {
	for _, entry := range entries {
		if entry.Path == relPath {
			return entry.Tags
		}
	}
	return nil
}

func ocrTextFor(entries []filesystem.FileEntry, relPath string) string {
	for _, entry := range entries {
		if entry.Path == relPath {
			return entry.OCRText
		}
	}
	return ""
}

func TestServeFile_AmpersandInDir(t *testing.T) {
	h := setupSpecialCharHandler(t)

	rr := serveAllWithChi(h, "GET", "/api/file/root-0/dir%26name/test.txt")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ampersand" {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
}

func TestServeFile_ApostropheInFilename(t *testing.T) {
	h := setupSpecialCharHandler(t)

	rr := serveAllWithChi(h, "GET", "/api/file/root-0/file%27name.txt")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "apostrophe" {
		t.Errorf("unexpected body: %s", rr.Body.String())
	}
}

func TestListDirectory_AmpersandInDir(t *testing.T) {
	h := setupSpecialCharHandler(t)

	rr := serveAllWithChi(h, "GET", "/api/tree/root-0/dir%26name")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestListDirectory_ApostropheInDir(t *testing.T) {
	h := setupSpecialCharHandler(t)

	rr := serveAllWithChi(h, "GET", "/api/tree/root-0/dir%27name")

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
