package handlers_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/lepinkainen/sampo/internal/analysis"
	"github.com/lepinkainen/sampo/internal/config"
	"github.com/lepinkainen/sampo/internal/filesystem"
	"github.com/lepinkainen/sampo/internal/server/handlers"
	"github.com/lepinkainen/sampo/internal/thumbnail"
)

func TestStartAnalyzeScanForceRecursesForReanalysis(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "nested", "tagged.jpg"), []byte("fake image"), 0644); err != nil {
		t.Fatal(err)
	}

	roots, err := filesystem.NewRootManager([]config.RootConfig{
		{Name: "test", Path: rootDir},
	})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := thumbnail.NewCache(filepath.Join(t.TempDir(), "thumbs"))
	if err != nil {
		t.Fatal(err)
	}

	logger := slog.Default()
	h := handlers.New(roots, cache, t.TempDir(), logger)
	coordinator := analysis.NewCoordinator(nil, nil, nil, nil, nil, nil, t.TempDir(), 1, 1, true, logger)
	scanner := analysis.NewScanner(coordinator, roots, 1, logger)
	t.Cleanup(scanner.Stop)
	h.SetAnalysisScanner(scanner)

	rr := serveAnalyzeScanWithChi(h, `{"rootId":"root-0","path":"/","force":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status analysis.ScanStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Total != 1 {
		t.Fatalf("total = %d, want 1 nested image included by force re-analysis", status.Total)
	}
}

func serveAnalyzeScanWithChi(h *handlers.Handler, body string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Post("/api/analyze/scan", h.StartAnalyzeScan)

	req := httptest.NewRequest(http.MethodPost, "/api/analyze/scan", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}
