package classification

import (
	"slices"
	"testing"
	"time"
)

func TestStorePutReplacesExistingTags(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	putClassificationResult(t, store, []TagScore{
		{Label: "legacy", Score: 0.9},
		{Label: "stale", Score: 0.8},
	})
	putClassificationResult(t, store, []TagScore{
		{Label: "fresh", Score: 0.95},
	})

	got, err := store.Get("root-0", "images/tagged.jpg")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil result")
	}
	if labels := tagLabels(got.Tags); !slices.Equal(labels, []string{"fresh"}) {
		t.Fatalf("tags = %v, want only fresh", labels)
	}

	dirTags, err := store.GetDirTags("root-0", "images")
	if err != nil {
		t.Fatalf("GetDirTags: %v", err)
	}
	if labels := tagLabels(dirTags["images/tagged.jpg"]); !slices.Equal(labels, []string{"fresh"}) {
		t.Fatalf("dir tags = %v, want only fresh", labels)
	}
}

func putClassificationResult(t *testing.T, store *Store, tags []TagScore) {
	t.Helper()
	if err := store.Put(&Result{
		RootID:    "root-0",
		RelPath:   "images/tagged.jpg",
		Mtime:     1,
		Size:      1,
		ModelVer:  "test-model",
		ScannedAt: time.Now().UTC(),
		Tags:      tags,
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}
}

func tagLabels(tags []TagScore) []string {
	labels := make([]string, len(tags))
	for i, tag := range tags {
		labels[i] = tag.Label
	}
	return labels
}
