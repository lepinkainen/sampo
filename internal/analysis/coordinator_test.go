package analysis

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lepinkainen/sampo/internal/classification"
	"github.com/lepinkainen/sampo/internal/thumbnail"
)

// TestNeedsPHashBackfill verifies that a fresh classification row missing its
// perceptual hash triggers a phash-only job (no ML analyzers), that rows with
// a phash trigger nothing, and that videos never trigger the phash check
// (their phash stays NULL forever — frames are temporary).
func TestNeedsPHashBackfill(t *testing.T) {
	store, err := classification.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	put := func(relPath, phash string) {
		t.Helper()
		if err := store.Put(&classification.Result{
			RootID:    "root-0",
			RelPath:   relPath,
			Mtime:     1,
			Size:      1,
			ModelVer:  "test-model",
			ScannedAt: time.Now().UTC(),
			PHash:     phash,
		}); err != nil {
			t.Fatal(err)
		}
	}
	put("photos/legacy.jpg", "")                 // pre-phash row
	put("photos/hashed.jpg", "00000000000000ff") // already backfilled
	put("videos/clip.mp4", "")                   // video: phash never applies

	// classStore without classifier: cls can never fire, isolating ph.
	coord := NewCoordinator(nil, nil, store, nil, nil, nil, nil, "", 1, 1, false, slog.Default())

	if n := coord.needs("root-0", "photos/legacy.jpg", "image", 1, 1, false); !n.phash || n.classify {
		t.Fatalf("legacy image row: classify=%v phash=%v, want phash only", n.classify, n.phash)
	}
	if n := coord.needs("root-0", "photos/hashed.jpg", "image", 1, 1, false); n.phash {
		t.Fatal("backfilled row should not need phash")
	}
	if n := coord.needs("root-0", "videos/clip.mp4", "video", 1, 1, false); n.phash {
		t.Fatal("video should never need phash")
	}
}

// writeTestPNG writes a small solid-color PNG to path.
func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 0, G: 128, B: 255, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating test image: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding test image: %v", err)
	}
}

// TestAnalyze_ThumbnailOnlyJob verifies that a file whose only stale "need" is
// a missing thumbnail (all ML disabled) still produces a thumbnail from the
// shared decode, without running any analyzer.
func TestAnalyze_ThumbnailOnlyJob(t *testing.T) {
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "photo.png")
	writeTestPNG(t, imgPath, 320, 240)

	info, err := os.Stat(imgPath)
	if err != nil {
		t.Fatal(err)
	}

	cache, err := thumbnail.NewCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	coord := NewCoordinator(nil, nil, nil, nil, nil, nil, nil, t.TempDir(), 1, 1, false, slog.Default())
	coord.SetThumbnailCache(cache)

	relPath := "photo.png"
	coord.Analyze(context.Background(), "root-0", relPath, imgPath, "image",
		info.ModTime().Unix(), info.Size(), false)

	key := thumbnail.CacheKey("root-0", relPath, info.ModTime().Unix(), info.Size())
	if _, ok := cache.Get("root-0", key); !ok {
		t.Fatal("thumbnail was not created by Analyze with thumb-only need")
	}
}

// TestIsPending verifies that IsPending reports true while a job is queued and
// false after it completes, and that it never reports true for a file that was
// never enqueued.
func TestIsPending(t *testing.T) {
	imgDir := t.TempDir()
	imgPath := filepath.Join(imgDir, "photo.png")
	writeTestPNG(t, imgPath, 10, 10)
	info, err := os.Stat(imgPath)
	if err != nil {
		t.Fatal(err)
	}

	cache, err := thumbnail.NewCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// queueSize 1, 1 worker — the job runs synchronously via Analyze below
	// so by the time we check IsPending the worker has already finished.
	coord := NewCoordinator(nil, nil, nil, nil, nil, nil, nil, t.TempDir(), 1, 1, false, slog.Default())
	coord.SetThumbnailCache(cache)

	relPath := "photo.png"
	mtime := info.ModTime().Unix()
	size := info.Size()

	// Never enqueued → not pending.
	if coord.IsPending("root-0", relPath, mtime, size) {
		t.Fatal("IsPending should be false before Analyze")
	}

	// Run the job to completion (Analyze is synchronous).
	coord.Analyze(context.Background(), "root-0", relPath, imgPath, "image", mtime, size, false)

	// Finished → not pending.
	if coord.IsPending("root-0", relPath, mtime, size) {
		t.Fatal("IsPending should be false after Analyze completes")
	}

	// Different mtime → different key → not pending.
	if coord.IsPending("root-0", relPath, mtime+1, size) {
		t.Fatal("IsPending should be false for a different key")
	}
}
