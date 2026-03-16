package handlers_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/filemanager/internal/config"
	"github.com/lepinkainen/filemanager/internal/filesystem"
	"github.com/lepinkainen/filemanager/internal/server/handlers"
	"github.com/lepinkainen/filemanager/internal/thumbnail"
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

	h := handlers.New(roots, cache, slog.Default())
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

	return handlers.New(roots, cache, slog.Default())
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
