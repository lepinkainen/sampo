package filesystem

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// buildWalkFixture creates a small tree with nested dirs, files, and a hidden dir.
func buildWalkFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dirs := []string{
		"a/b/c",
		"a/d",
		"e",
		".hidden/sub",
	}
	files := []string{
		"top.txt",
		"a/one.txt",
		"a/b/two.txt",
		"a/b/c/three.txt",
		"a/d/four.txt",
		"e/five.txt",
		".hidden/secret.txt",
		".hidden/sub/deep.txt",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestWalkDirParallelMatchesWalkDir(t *testing.T) {
	root := buildWalkFixture(t)

	var want []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		want = append(want, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var got []string
	err = WalkDirParallel(context.Background(), root, func(path string, d fs.DirEntry) error {
		mu.Lock()
		got = append(got, path)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(want)
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("entry count mismatch: got %d want %d\ngot: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestWalkDirParallelSkipDir(t *testing.T) {
	root := buildWalkFixture(t)

	var mu sync.Mutex
	var got []string
	err := WalkDirParallel(context.Background(), root, func(path string, d fs.DirEntry) error {
		if d.IsDir() && d.Name() == "a" {
			return filepath.SkipDir
		}
		mu.Lock()
		got = append(got, path)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range got {
		if rel, _ := filepath.Rel(root, p); rel == "a" || strings.HasPrefix(rel, "a"+string(filepath.Separator)) {
			t.Errorf("visited pruned path %q", p)
		}
	}
}

func TestWalkDirParallelSkipAll(t *testing.T) {
	root := buildWalkFixture(t)

	var count atomic.Int32
	err := WalkDirParallel(context.Background(), root, func(path string, d fs.DirEntry) error {
		if count.Add(1) >= 3 {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		t.Fatalf("SkipAll should not surface as error, got %v", err)
	}
}

func TestWalkDirParallelContextCancel(t *testing.T) {
	root := buildWalkFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	var count atomic.Int32
	err := WalkDirParallel(ctx, root, func(path string, d fs.DirEntry) error {
		if count.Add(1) >= 2 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestWalkDirParallelPropagatesError(t *testing.T) {
	root := buildWalkFixture(t)

	sentinel := errors.New("boom")
	err := WalkDirParallel(context.Background(), root, func(path string, d fs.DirEntry) error {
		if filepath.Base(path) == "two.txt" {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

func TestWalkDirParallelNonDirRoot(t *testing.T) {
	root := buildWalkFixture(t)
	file := filepath.Join(root, "top.txt")

	var got []string
	err := WalkDirParallel(context.Background(), file, func(path string, d fs.DirEntry) error {
		got = append(got, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != file {
		t.Fatalf("expected single visit of %q, got %v", file, got)
	}
}
