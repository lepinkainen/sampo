package ocr

import (
	"database/sql"
	"slices"
	"strings"
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

func putTestResult(t *testing.T, store *Store, relPath, text string) {
	t.Helper()
	err := store.Put(&Result{
		RootID:    "root-0",
		RelPath:   relPath,
		Mtime:     123,
		Size:      456,
		ModelVer:  "test-model",
		ScannedAt: time.Now().UTC(),
		Text:      text,
		Blocks: []TextBlock{{
			Text:       text,
			Confidence: 0.9,
		}},
	})
	if err != nil {
		t.Fatalf("Put(%q): %v", relPath, err)
	}
}

func TestStoreNormalizesRootAndNestedRelPaths(t *testing.T) {
	store := newTestStore(t)
	putTestResult(t, store, "/root.jpg", "root words")
	putTestResult(t, store, "/album/nested.jpg", "nested words")

	for _, relPath := range []string{"root.jpg", "/root.jpg"} {
		got, err := store.Get("root-0", relPath)
		if err != nil {
			t.Fatalf("Get(%q): %v", relPath, err)
		}
		if got == nil {
			t.Fatalf("Get(%q) = nil", relPath)
		}
		if got.RelPath != "root.jpg" {
			t.Fatalf("Get(%q).RelPath = %q, want root.jpg", relPath, got.RelPath)
		}
	}

	if store.IsStale("root-0", "/root.jpg", 123, 456, "test-model") {
		t.Fatal("IsStale with leading slash = true, want false")
	}
	if store.IsStale("root-0", "album/nested.jpg", 123, 456, "test-model") {
		t.Fatal("IsStale nested = true, want false")
	}

	rootText, err := store.GetDirText("root-0", "/")
	if err != nil {
		t.Fatalf("GetDirText(root): %v", err)
	}
	if rootText["root.jpg"] != "root words" {
		t.Fatalf("root dir text = %#v, want root.jpg", rootText)
	}
	if _, ok := rootText["album/nested.jpg"]; ok {
		t.Fatalf("root dir text included nested file: %#v", rootText)
	}

	nestedText, err := store.GetDirText("root-0", "/album")
	if err != nil {
		t.Fatalf("GetDirText(album): %v", err)
	}
	if nestedText["album/nested.jpg"] != "nested words" {
		t.Fatalf("nested dir text = %#v, want album/nested.jpg", nestedText)
	}

	text, err := store.GetText("root-0", "/album/nested.jpg")
	if err != nil {
		t.Fatalf("GetText(nested): %v", err)
	}
	if text != "nested words" {
		t.Fatalf("GetText(nested) = %q, want nested words", text)
	}

	paths, err := store.SearchByText("root-0", "/", "words")
	if err != nil {
		t.Fatalf("SearchByText(root): %v", err)
	}
	slices.Sort(paths)
	want := []string{"album/nested.jpg", "root.jpg"}
	if !slices.Equal(paths, want) {
		t.Fatalf("SearchByText(root) = %#v, want %#v", paths, want)
	}

	paths, err = store.SearchByText("root-0", "/album", "nested")
	if err != nil {
		t.Fatalf("SearchByText(album): %v", err)
	}
	if !slices.Equal(paths, []string{"album/nested.jpg"}) {
		t.Fatalf("SearchByText(album) = %#v, want album/nested.jpg", paths)
	}
}

func TestStoreMigratesLegacyLeadingSlashRelPaths(t *testing.T) {
	cacheDir := t.TempDir()
	store, err := NewStore(cacheDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	oldScan := time.Now().Add(-time.Hour).UTC()
	newScan := time.Now().UTC()
	insertOCRRow(t, store.db, "/legacy.jpg", "legacy text", oldScan)
	insertOCRRow(t, store.db, "/dupe.jpg", "old duplicate", oldScan)
	insertOCRRow(t, store.db, "dupe.jpg", "new duplicate", newScan)
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	store, err = NewStore(cacheDir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	legacy, err := store.Get("root-0", "/legacy.jpg")
	if err != nil {
		t.Fatalf("Get legacy: %v", err)
	}
	if legacy == nil || legacy.RelPath != "legacy.jpg" || legacy.Text != "legacy text" {
		t.Fatalf("legacy row = %#v, want canonical legacy.jpg", legacy)
	}

	dupe, err := store.Get("root-0", "/dupe.jpg")
	if err != nil {
		t.Fatalf("Get dupe: %v", err)
	}
	if dupe == nil || dupe.RelPath != "dupe.jpg" || dupe.Text != "new duplicate" {
		t.Fatalf("dupe row = %#v, want canonical newer row", dupe)
	}

	rows, err := store.db.Query(`SELECT rel_path FROM ocr`)
	if err != nil {
		t.Fatalf("query paths: %v", err)
	}
	defer func() { _ = rows.Close() }()

	counts := map[string]int{}
	for rows.Next() {
		var relPath string
		if err := rows.Scan(&relPath); err != nil {
			t.Fatalf("scan path: %v", err)
		}
		if strings.HasPrefix(relPath, "/") {
			t.Fatalf("legacy leading slash row survived: %q", relPath)
		}
		counts[relPath]++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if counts["dupe.jpg"] != 1 {
		t.Fatalf("dupe row count = %d, want 1 (all counts %#v)", counts["dupe.jpg"], counts)
	}
}

func insertOCRRow(t *testing.T, db *sql.DB, relPath, text string, scannedAt time.Time) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO ocr (root_id, rel_path, mtime, size, model_ver, scanned_at, text, blocks)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"root-0", relPath, 123, 456, "test-model", scannedAt, text, "[]",
	)
	if err != nil {
		t.Fatalf("insert %q: %v", relPath, err)
	}
}
