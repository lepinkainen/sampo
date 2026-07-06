package analysis

import (
	"log/slog"
	"testing"
	"time"

	"github.com/lepinkainen/sampo/internal/classification"
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
	coord := NewCoordinator(nil, nil, store, nil, nil, nil, "", 1, 1, false, slog.Default())

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
