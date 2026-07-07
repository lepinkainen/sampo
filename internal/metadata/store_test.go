package metadata

import (
	"slices"
	"testing"
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

func TestStorePutGetRoundTrip(t *testing.T) {
	store := newTestStore(t)

	d, ok, err := store.Get("root-0", "photos/a.jpg")
	if err != nil {
		t.Fatalf("Get before Put: %v", err)
	}
	if ok {
		t.Fatal("expected miss before Put")
	}

	if err := store.Put("root-0", "photos/a.jpg", 100, Dims{Width: 1920, Height: 1080}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	d, ok, err = store.Get("root-0", "photos/a.jpg")
	if err != nil || !ok {
		t.Fatalf("Get after Put: ok=%v err=%v", ok, err)
	}
	if d.Width != 1920 || d.Height != 1080 {
		t.Fatalf("dims = %dx%d, want 1920x1080", d.Width, d.Height)
	}
	if d.Duration != 0 {
		t.Fatalf("duration = %v, want 0 for image", d.Duration)
	}
}

func TestStoreGetMany(t *testing.T) {
	store := newTestStore(t)

	if err := store.Put("root-0", "photos/a.jpg", 1, Dims{Width: 1920, Height: 1080}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put("root-0", "videos/v.mp4", 1, Dims{Width: 1280, Height: 720, Duration: 12.5}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put("root-1", "photos/a.jpg", 1, Dims{Width: 640, Height: 480}); err != nil {
		t.Fatalf("Put other root: %v", err)
	}

	dims, err := store.GetMany("root-0", []string{"photos/a.jpg", "videos/v.mp4", "missing.jpg"})
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}
	if len(dims) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(dims), dims)
	}
	if d := dims["photos/a.jpg"]; d.Width != 1920 || d.Height != 1080 {
		t.Fatalf("photos/a.jpg = %+v, want 1920x1080", d)
	}
	if d := dims["videos/v.mp4"]; d.Width != 1280 || d.Duration != 12.5 {
		t.Fatalf("videos/v.mp4 = %+v, want 1280x720 dur 12.5", d)
	}

	dims, err = store.GetMany("root-0", nil)
	if err != nil {
		t.Fatalf("GetMany empty: %v", err)
	}
	if len(dims) != 0 {
		t.Fatalf("GetMany empty returned %+v", dims)
	}
}

func TestStorePutReplacesWithNewMtime(t *testing.T) {
	store := newTestStore(t)

	if err := store.Put("root-0", "v.mp4", 1, Dims{Width: 1280, Height: 720, Duration: 10}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put("root-0", "v.mp4", 2, Dims{Width: 1920, Height: 1080, Duration: 20}); err != nil {
		t.Fatalf("Put replace: %v", err)
	}

	d, ok, _ := store.Get("root-0", "v.mp4")
	if !ok || d.Width != 1920 || d.Height != 1080 || d.Duration != 20 {
		t.Fatalf("after replace got %+v ok=%v", d, ok)
	}
	if store.IsStale("root-0", "v.mp4", 2) {
		t.Fatal("IsStale should be false for current mtime")
	}
	if !store.IsStale("root-0", "v.mp4", 99) {
		t.Fatal("IsStale should be true for changed mtime")
	}
}

func TestStoreGetDirDimensionsDirectChildrenOnly(t *testing.T) {
	store := newTestStore(t)

	puts := []struct {
		path          string
		width, height int
		duration      float64
	}{
		{"/photos/a.jpg", 1000, 2000, 0},
		{"/photos/b.jpg", 640, 480, 0},
		{"/photos/nested/c.jpg", 50, 50, 0}, // nested: must be excluded
		{"/other/d.jpg", 10, 10, 0},         // sibling dir: excluded
	}
	for _, p := range puts {
		if err := store.Put("root-0", p.path, 1, Dims{Width: p.width, Height: p.height, Duration: p.duration}); err != nil {
			t.Fatalf("Put %s: %v", p.path, err)
		}
	}

	got, err := store.GetDirDimensions("root-0", "/photos")
	if err != nil {
		t.Fatalf("GetDirDimensions: %v", err)
	}

	wantPaths := []string{"/photos/a.jpg", "/photos/b.jpg"}
	gotPaths := make([]string, 0, len(got))
	for p := range got {
		gotPaths = append(gotPaths, p)
	}
	slices.Sort(gotPaths)
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("paths = %v, want %v", gotPaths, wantPaths)
	}
	if got["/photos/a.jpg"].Width != 1000 || got["/photos/a.jpg"].Height != 2000 {
		t.Fatalf("a.jpg dims = %+v", got["/photos/a.jpg"])
	}
}

func TestStoreGetDirDimensionsRootScope(t *testing.T) {
	store := newTestStore(t)
	_ = store.Put("root-0", "/a.jpg", 1, Dims{Width: 1, Height: 1})
	_ = store.Put("root-0", "/sub/b.jpg", 1, Dims{Width: 2, Height: 2})

	// dirPath "/" -> dirPrefix "/" -> LIKE "/%" matches both; direct-child
	// filter keeps only the top-level file.
	got, err := store.GetDirDimensions("root-0", "/")
	if err != nil {
		t.Fatalf("GetDirDimensions root: %v", err)
	}
	if len(got) != 1 || got["/a.jpg"].Width != 1 {
		t.Fatalf("root direct children = %+v, want only /a.jpg", got)
	}
}

func TestStoreDeletePathFileAndSubtree(t *testing.T) {
	store := newTestStore(t)
	_ = store.Put("root-0", "/sub/a.jpg", 1, Dims{Width: 1, Height: 1})
	_ = store.Put("root-0", "/sub/deep/b.jpg", 1, Dims{Width: 2, Height: 2})
	_ = store.Put("root-0", "/sub2/c.jpg", 1, Dims{Width: 3, Height: 3})

	// Subtree delete: drops everything under /sub
	if err := store.DeletePath("root-0", "/sub"); err != nil {
		t.Fatalf("DeletePath subtree: %v", err)
	}
	if _, ok, _ := store.Get("root-0", "/sub/a.jpg"); ok {
		t.Fatal("/sub/a.jpg should be deleted")
	}
	if _, ok, _ := store.Get("root-0", "/sub/deep/b.jpg"); ok {
		t.Fatal("/sub/deep/b.jpg should be deleted")
	}
	if _, ok, _ := store.Get("root-0", "/sub2/c.jpg"); !ok {
		t.Fatal("/sub2/c.jpg should remain")
	}

	// Single-file delete.
	if err := store.DeletePath("root-0", "/sub2/c.jpg"); err != nil {
		t.Fatalf("DeletePath file: %v", err)
	}
	if _, ok, _ := store.Get("root-0", "/sub2/c.jpg"); ok {
		t.Fatal("/sub2/c.jpg should be deleted")
	}
}

func TestStoreDeletePathRefusesRoot(t *testing.T) {
	store := newTestStore(t)
	if err := store.DeletePath("root-0", ""); err == nil {
		t.Fatal("DeletePath('') should error")
	}
	if err := store.DeletePath("root-0", "/"); err == nil {
		t.Fatal("DeletePath('/') should error")
	}
}

func TestStoreListPaths(t *testing.T) {
	store := newTestStore(t)
	_ = store.Put("root-0", "/keep/a.jpg", 1, Dims{Width: 1, Height: 1})
	_ = store.Put("root-0", "/keep/b.jpg", 1, Dims{Width: 2, Height: 2})
	_ = store.Put("root-0", "/drop/c.jpg", 1, Dims{Width: 3, Height: 3})

	paths, err := store.ListPaths("root-0", "/keep")
	if err != nil {
		t.Fatalf("ListPaths: %v", err)
	}
	slices.Sort(paths)
	if !slices.Equal(paths, []string{"/keep/a.jpg", "/keep/b.jpg"}) {
		t.Fatalf("paths = %v", paths)
	}
}
