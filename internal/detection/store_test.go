package detection

import (
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStorePragmasApplied(t *testing.T) {
	store := newTestStore(t)

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
}

func putDetection(t *testing.T, store *Store, relPath string) {
	t.Helper()
	if err := store.Put(&Result{
		RootID:    "root-0",
		RelPath:   relPath,
		Mtime:     1,
		Size:      100,
		HasPerson: true,
		ModelVer:  "test-model",
		ScannedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Put(%q): %v", relPath, err)
	}
}

func TestStoreDeletePath(t *testing.T) {
	store := newTestStore(t)
	putDetection(t, store, "album/a.jpg")
	putDetection(t, store, "album/sub/b.jpg")
	putDetection(t, store, "other.jpg")

	if err := store.DeletePath("root-0", "album/a.jpg"); err != nil {
		t.Fatalf("DeletePath: %v", err)
	}
	if _, err := store.GetDetection("root-0", "album/a.jpg"); err == nil {
		t.Fatal("detection should be gone after delete")
	}

	// Subtree delete.
	if err := store.DeletePath("root-0", "album"); err != nil {
		t.Fatalf("DeletePath subtree: %v", err)
	}
	if _, err := store.GetDetection("root-0", "album/sub/b.jpg"); err == nil {
		t.Fatal("nested detection should be gone after subtree delete")
	}
	if hasPerson, err := store.GetDetection("root-0", "other.jpg"); err != nil || !hasPerson {
		t.Fatalf("sibling detection should survive: %v, %v", hasPerson, err)
	}

	// Root paths refused.
	for _, p := range []string{"", "/"} {
		if err := store.DeletePath("root-0", p); err == nil {
			t.Fatalf("DeletePath(%q) should refuse", p)
		}
	}
}
