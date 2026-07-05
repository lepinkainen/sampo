package classification

import (
	"fmt"
	"slices"
	"sync"
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

func TestStoreDeletePathRemovesFileFromDuplicates(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	putFile(t, store, "images/a.jpg", "samehash")
	putFile(t, store, "images/b.jpg", "samehash")

	groups, err := store.FindDuplicates("root-0", "images")
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(groups) != 1 || len(groups[0].Files) != 2 {
		t.Fatalf("groups = %+v, want 1 group with 2 files", groups)
	}

	if err := store.DeletePath("root-0", "images/b.jpg"); err != nil {
		t.Fatalf("DeletePath: %v", err)
	}

	groups, err = store.FindDuplicates("root-0", "images")
	if err != nil {
		t.Fatalf("FindDuplicates after delete: %v", err)
	}
	if len(groups) != 0 {
		t.Fatalf("groups after delete = %+v, want none", groups)
	}

	// The surviving file's row is untouched.
	got, err := store.Get("root-0", "images/a.jpg")
	if err != nil || got == nil {
		t.Fatalf("Get survivor: %v, %v", got, err)
	}
	// The deleted file's tags are gone too.
	tags, err := store.GetFileTags("root-0", "images/b.jpg")
	if err != nil {
		t.Fatalf("GetFileTags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("tags after delete = %v, want none", tags)
	}
}

func TestStoreDeletePathRemovesSubtree(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	putFile(t, store, "images/sub/a.jpg", "h1")
	putFile(t, store, "images/sub/b.jpg", "h2")
	putFile(t, store, "images/other.jpg", "h3")

	if err := store.DeletePath("root-0", "images/sub"); err != nil {
		t.Fatalf("DeletePath: %v", err)
	}

	checksums, err := store.GetDirChecksums("root-0", "images/sub")
	if err != nil {
		t.Fatalf("GetDirChecksums: %v", err)
	}
	if len(checksums) != 0 {
		t.Fatalf("checksums under deleted dir = %v, want none", checksums)
	}
	got, err := store.Get("root-0", "images/other.jpg")
	if err != nil || got == nil {
		t.Fatalf("sibling file should survive: %v, %v", got, err)
	}
}

func TestStoreDeletePathRefusesRoot(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, p := range []string{"", "/"} {
		if err := store.DeletePath("root-0", p); err == nil {
			t.Fatalf("DeletePath(%q) should refuse", p)
		}
	}
}

func TestStoreDeletePathEscapesLikeWildcards(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	putFile(t, store, "backup_old/a.jpg", "h1")
	putFile(t, store, "backupXold/b.jpg", "h2")

	if err := store.DeletePath("root-0", "backup_old"); err != nil {
		t.Fatalf("DeletePath: %v", err)
	}

	if got, err := store.Get("root-0", "backup_old/a.jpg"); err != nil || got != nil {
		t.Fatalf("target row should be gone: %v, %v", got, err)
	}
	// The underscore must match literally, not as a LIKE wildcard.
	if got, err := store.Get("root-0", "backupXold/b.jpg"); err != nil || got == nil {
		t.Fatalf("sibling with wildcard-lookalike name should survive: %v, %v", got, err)
	}
}

func TestStoreDeletePathNormalizesLeadingSlash(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	putFile(t, store, "images/a.jpg", "h1")
	putFile(t, store, "/images/b.jpg", "h2")

	if err := store.DeletePath("root-0", "/images/a.jpg"); err != nil {
		t.Fatalf("DeletePath: %v", err)
	}
	if got, err := store.Get("root-0", "images/a.jpg"); err != nil || got != nil {
		t.Fatalf("row should be gone after leading-slash delete: %v, %v", got, err)
	}
	if err := store.DeletePath("root-0", "images/b.jpg"); err != nil {
		t.Fatalf("DeletePath leading stored row: %v", err)
	}
	if got, err := store.Get("root-0", "/images/b.jpg"); err != nil || got != nil {
		t.Fatalf("leading-slash row should be gone after delete: %v, %v", got, err)
	}
}

func putFile(t *testing.T, store *Store, relPath, sha string) {
	t.Helper()
	if err := store.Put(&Result{
		RootID:    "root-0",
		RelPath:   relPath,
		Mtime:     1,
		Size:      100,
		ModelVer:  "test-model",
		ScannedAt: time.Now().UTC(),
		SHA256:    sha,
		Tags:      []TagScore{{Label: "tag", Score: 0.9}},
	}); err != nil {
		t.Fatalf("Put %s: %v", relPath, err)
	}
}

func TestStorePragmasApplied(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var journalMode string
	if err := store.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("querying journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal (DSN pragma not applied)", journalMode)
	}

	var busyTimeout int
	if err := store.db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("querying busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000 (DSN pragma not applied)", busyTimeout)
	}

	var foreignKeys int
	if err := store.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("querying foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1 (DSN pragma not applied)", foreignKeys)
	}
}

func TestStoreConcurrentPutsDoNotReturnBusy(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	const writers = 20
	const putsPerWriter = 5
	errs := make(chan error, writers*putsPerWriter)
	var wg sync.WaitGroup
	for w := range writers {
		wg.Go(func() {
			for p := range putsPerWriter {
				errs <- store.Put(&Result{
					RootID:    "root-0",
					RelPath:   fmt.Sprintf("images/w%d-p%d.jpg", w, p),
					Mtime:     1,
					Size:      1,
					ModelVer:  "test-model",
					ScannedAt: time.Now().UTC(),
					Tags:      []TagScore{{Label: "a", Score: 0.9}, {Label: "b", Score: 0.8}},
				})
			}
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Put: %v", err)
		}
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
