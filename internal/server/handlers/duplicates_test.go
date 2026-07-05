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
	"github.com/lepinkainen/sampo/internal/server/handlers"
	"github.com/lepinkainen/sampo/internal/thumbnail"
)

func TestFindDuplicatesPrunesMissingCandidates(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "photos"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "photos", "orig.jpg"), []byte("same bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	roots, err := filesystem.NewRootManager([]config.RootConfig{{Name: "test", Path: dir}})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := thumbnail.NewCache(filepath.Join(dir, ".cache"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := classification.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, relPath := range []string{"/photos/orig.jpg", "/photos/ghost.jpg"} {
		if err := store.Put(&classification.Result{
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
	}

	h := handlers.New(roots, cache, t.TempDir(), slog.Default())
	h.SetClassification(store, nil, nil)

	r := chi.NewRouter()
	r.Get("/api/duplicates/{rootID}/*", h.FindDuplicates)
	req := httptest.NewRequest("GET", "/api/duplicates/root-0//photos", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var res struct {
		Groups []classification.DuplicateGroup `json:"groups"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if len(res.Groups) != 0 {
		t.Fatalf("groups = %+v, want missing candidate filtered", res.Groups)
	}
	if got, err := store.Get("root-0", "/photos/ghost.jpg"); err != nil || got != nil {
		t.Fatalf("ghost cache row should be pruned: %v, %v", got, err)
	}
	if got, err := store.Get("root-0", "/photos/orig.jpg"); err != nil || got == nil {
		t.Fatalf("existing cache row should remain: %v, %v", got, err)
	}
}
