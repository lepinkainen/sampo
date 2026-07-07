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

func TestStorePHashRoundtrip(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Put(&Result{
		RootID: "root-0", RelPath: "images/a.jpg", Mtime: 1, Size: 100,
		ModelVer: "test-model", ScannedAt: time.Now().UTC(),
		SHA256: "h1", PHash: "d0d14f6a0e2d3d71", Width: 4032, Height: 3024,
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get("root-0", "images/a.jpg")
	if err != nil || got == nil {
		t.Fatalf("Get: %v, %v", got, err)
	}
	if got.PHash != "d0d14f6a0e2d3d71" || got.Width != 4032 || got.Height != 3024 {
		t.Fatalf("roundtrip = phash %q %dx%d", got.PHash, got.Width, got.Height)
	}
}

func TestStoreMigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	putFile(t, store, "images/a.jpg", "h1")
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopening the same database must re-run migrations without error and
	// keep existing rows.
	store, err = NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reopen: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	got, err := store.Get("root-0", "images/a.jpg")
	if err != nil || got == nil {
		t.Fatalf("Get after reopen: %v, %v", got, err)
	}
}

func TestStoreNeedsPHashAndUpdate(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if store.NeedsPHash("root-0", "images/a.jpg") {
		t.Fatal("NeedsPHash should be false for a missing row")
	}

	putFile(t, store, "images/a.jpg", "h1") // legacy row, no phash
	if !store.NeedsPHash("root-0", "images/a.jpg") {
		t.Fatal("NeedsPHash should be true for a NULL-phash row")
	}

	if err := store.UpdatePHash("root-0", "images/a.jpg", "00000000000000ff", 800, 600); err != nil {
		t.Fatalf("UpdatePHash: %v", err)
	}
	if store.NeedsPHash("root-0", "images/a.jpg") {
		t.Fatal("NeedsPHash should be false after UpdatePHash")
	}

	// UpdatePHash must not touch tags, mtime, or model version.
	got, err := store.Get("root-0", "images/a.jpg")
	if err != nil || got == nil {
		t.Fatalf("Get: %v, %v", got, err)
	}
	if got.Mtime != 1 || got.ModelVer != "test-model" || len(got.Tags) != 1 {
		t.Fatalf("UpdatePHash side effects: mtime=%d modelVer=%q tags=%v", got.Mtime, got.ModelVer, got.Tags)
	}
	if got.PHash != "00000000000000ff" || got.Width != 800 || got.Height != 600 {
		t.Fatalf("UpdatePHash values: phash=%q %dx%d", got.PHash, got.Width, got.Height)
	}
}

func putPHashFile(t *testing.T, store *Store, relPath, sha, phash string, width, height int) {
	t.Helper()
	if err := store.Put(&Result{
		RootID: "root-0", RelPath: relPath, Mtime: 1, Size: 100,
		ModelVer: "test-model", ScannedAt: time.Now().UTC(),
		SHA256: sha, PHash: phash, Width: width, Height: height,
	}); err != nil {
		t.Fatalf("Put %s: %v", relPath, err)
	}
}

func TestStoreFindSimilar(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Near pair (1 bit apart), original bigger than the resized copy.
	putPHashFile(t, store, "images/orig.png", "sha-orig", "00000000000000f0", 4032, 3024)
	putPHashFile(t, store, "images/resized.jpg", "sha-resized", "00000000000000f1", 1920, 1440)
	// Byte-identical pair: belongs to FindDuplicates, not FindSimilar.
	putPHashFile(t, store, "images/copy1.jpg", "sha-same", "000000000000ff00", 800, 600)
	putPHashFile(t, store, "images/copy2.jpg", "sha-same", "000000000000ff00", 800, 600)
	// Unrelated image far from everything.
	putPHashFile(t, store, "images/other.jpg", "sha-other", "ffffffffffffffff", 800, 600)

	groups, err := store.FindSimilar("root-0", "images", 7)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("similar groups = %+v, want exactly the near pair", groups)
	}
	g := groups[0]
	if g.HashType != "phash" || len(g.Files) != 2 {
		t.Fatalf("group = %+v", g)
	}
	if g.Keeper == nil || *g.Keeper != 0 || g.Files[0].Path != "images/orig.png" {
		t.Fatalf("keeper should be the high-res original: %+v", g)
	}
	if g.Files[1].Similarity != DistanceToSimilarity(1) {
		t.Fatalf("similarity = %d", g.Files[1].Similarity)
	}

	// The byte-identical pair still shows up as an exact duplicate group.
	exact, err := store.FindDuplicates("root-0", "images")
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(exact) != 1 || len(exact[0].Files) != 2 {
		t.Fatalf("exact groups = %+v, want the sha-same pair", exact)
	}
}

func TestStoreFindSimilarQualityTie(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	putPHashFile(t, store, "scans/poster.png", "sha-png", "00000000000000f0", 3600, 2400)
	putPHashFile(t, store, "scans/poster.webp", "sha-webp", "00000000000000f1", 3600, 2400)

	groups, err := store.FindSimilar("root-0", "scans", 7)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Keeper != nil {
		t.Fatalf("keeper = %v, want nil on same-resolution tie", *groups[0].Keeper)
	}
}

func TestStoreFindDuplicatesByCRC32(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.PutFilenameCRC32Batch("root-0", []FilenameCRC32Entry{
		{RelPath: "videos/a[DEADBEEF].mp4", Mtime: 1, Size: 1000, CRC32: "DEADBEEF"},
		{RelPath: "videos/b[DEADBEEF].mkv", Mtime: 2, Size: 1000, CRC32: "DEADBEEF"},
		{RelPath: "videos/c[12345678].mp4", Mtime: 3, Size: 500, CRC32: "12345678"},
	}); err != nil {
		t.Fatalf("PutFilenameCRC32Batch: %v", err)
	}

	groups, err := store.FindDuplicates("root-0", "videos")
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %+v, want 1 CRC32 group", groups)
	}
	g := groups[0]
	if g.HashType != "crc32" || g.Hash != "DEADBEEF" || len(g.Files) != 2 {
		t.Fatalf("group = %+v, want crc32/DEADBEEF with 2 files", g)
	}
}

func TestStoreFindDuplicatesCRC32ExcludesSHA256Files(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Two videos with filename CRC32, no SHA256 → CRC32 group.
	if err := store.PutFilenameCRC32Batch("root-0", []FilenameCRC32Entry{
		{RelPath: "v/a[DEADBEEF].mp4", Mtime: 1, Size: 1000, CRC32: "DEADBEEF"},
		{RelPath: "v/b[DEADBEEF].mkv", Mtime: 2, Size: 1000, CRC32: "DEADBEEF"},
	}); err != nil {
		t.Fatalf("PutFilenameCRC32Batch: %v", err)
	}
	// A third file with the same CRC32 but also a SHA256 (classified image)
	// must NOT appear in the CRC32 group — it belongs to SHA256 dedup.
	putFile(t, store, "v/c[DEADBEEF].jpg", "sha-real")

	groups, err := store.FindDuplicates("root-0", "v")
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %+v, want only the CRC32 video pair", groups)
	}
	if groups[0].HashType != "crc32" || len(groups[0].Files) != 2 {
		t.Fatalf("group = %+v, want crc32 with 2 files (image excluded)", groups[0])
	}
}

func TestStorePutPreservesFilenameCRC32(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Seed a filename-derived CRC32.
	if err := store.PutFilenameCRC32Batch("root-0", []FilenameCRC32Entry{
		{RelPath: "v/clip[ABCD1234].mp4", Mtime: 1, Size: 500, CRC32: "ABCD1234"},
	}); err != nil {
		t.Fatalf("PutFilenameCRC32Batch: %v", err)
	}

	// Simulate classification writing tags but blank CRC32 (videos blank both
	// hashes because frames are temporary, not the original file).
	if err := store.Put(&Result{
		RootID:    "root-0",
		RelPath:   "v/clip[ABCD1234].mp4",
		Mtime:     1,
		Size:      500,
		ModelVer:  "clip-v1",
		ScannedAt: time.Now().UTC(),
		Tags:      []TagScore{{Label: "video", Score: 0.9}},
	}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Get("root-0", "v/clip[ABCD1234].mp4")
	if err != nil || got == nil {
		t.Fatalf("Get: %v, %v", got, err)
	}
	if got.CRC32 != "ABCD1234" {
		t.Fatalf("CRC32 = %q, want ABCD1234 preserved through classification Put", got.CRC32)
	}
	if len(got.Tags) != 1 || got.Tags[0].Label != "video" {
		t.Fatalf("tags = %+v, want classification tags preserved", got.Tags)
	}
	if got.ModelVer != "clip-v1" {
		t.Fatalf("model_ver = %q, want clip-v1 (classification overwrites filename sentinel)", got.ModelVer)
	}
}

func TestStorePutFilenameCRC32BatchEmpty(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.PutFilenameCRC32Batch("root-0", nil); err != nil {
		t.Fatalf("PutFilenameCRC32Batch(nil): %v", err)
	}
	if err := store.PutFilenameCRC32Batch("root-0", []FilenameCRC32Entry{}); err != nil {
		t.Fatalf("PutFilenameCRC32Batch(empty): %v", err)
	}
}
