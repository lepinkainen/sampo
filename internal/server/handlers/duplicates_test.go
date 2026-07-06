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

func TestFindDuplicatesSimilarParam(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "photos"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"orig.png":    "original bytes",
		"resized.jpg": "resized bytes",
	} {
		if err := os.WriteFile(filepath.Join(dir, "photos", name), []byte(content), 0644); err != nil {
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
	store, err := classification.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Near pair: 1 Hamming bit apart, original higher resolution.
	files := []struct {
		relPath string
		sha     string
		phash   string
		w, h    int
	}{
		{"/photos/orig.png", "sha-a", "00000000000000f0", 4032, 3024},
		{"/photos/resized.jpg", "sha-b", "00000000000000f1", 1920, 1440},
	}
	for _, f := range files {
		if err := store.Put(&classification.Result{
			RootID:    "root-0",
			RelPath:   f.relPath,
			Mtime:     1,
			Size:      10,
			ModelVer:  "test-model",
			ScannedAt: time.Now().UTC(),
			SHA256:    f.sha,
			PHash:     f.phash,
			Width:     f.w,
			Height:    f.h,
		}); err != nil {
			t.Fatal(err)
		}
	}

	h := handlers.New(roots, cache, t.TempDir(), slog.Default())
	h.SetClassification(store, nil, nil)

	r := chi.NewRouter()
	r.Get("/api/duplicates/{rootID}/*", h.FindDuplicates)

	get := func(url string) []classification.DuplicateGroup {
		t.Helper()
		req := httptest.NewRequest("GET", url, nil)
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
		return res.Groups
	}

	// Default request: exact only — no sha256 dupes here, so empty.
	if groups := get("/api/duplicates/root-0//photos"); len(groups) != 0 {
		t.Fatalf("default groups = %+v, want none without similar=true", groups)
	}

	// similar=true surfaces the phash group with keeper metadata.
	groups := get("/api/duplicates/root-0//photos?similar=true&threshold=88")
	if len(groups) != 1 {
		t.Fatalf("similar groups = %+v, want 1", groups)
	}
	g := groups[0]
	if g.HashType != "phash" || len(g.Files) != 2 {
		t.Fatalf("group = %+v", g)
	}
	if g.Keeper == nil || *g.Keeper != 0 || g.Files[0].Path != "/photos/orig.png" {
		t.Fatalf("keeper should be the high-res original: %+v", g)
	}
	if g.Files[1].Similarity == 0 || g.Files[1].Width != 1920 {
		t.Fatalf("file metadata missing: %+v", g.Files[1])
	}

	// A too-strict threshold (clamped from an out-of-range value) drops the pair.
	if groups := get("/api/duplicates/root-0//photos?similar=true&threshold=150"); len(groups) != 0 {
		t.Fatalf("threshold=150 clamps to 100 (0 bits), got %+v", groups)
	}
}
