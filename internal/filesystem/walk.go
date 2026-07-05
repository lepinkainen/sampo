package filesystem

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

// walkConcurrency bounds concurrent directory reads during a parallel walk.
// Matches the semaphore capacity used for thumbnail detection in tree.go —
// high enough to hide network-mount latency, low enough not to hammer NFS/SMB.
const walkConcurrency = 16

// forEachLimit runs fn(i) for i in [0, n) with at most walkConcurrency
// invocations in flight and waits for all of them to finish. fn must
// coordinate its own writes (index-addressed slots are the usual pattern).
func forEachLimit(n int, fn func(i int)) {
	var g errgroup.Group
	g.SetLimit(walkConcurrency)
	for i := range n {
		g.Go(func() error {
			fn(i)
			return nil
		})
	}
	_ = g.Wait() // fn reports failures through its own state, never errors
}

// WalkDirParallel walks the tree rooted at root, calling fn for each file or
// directory. Unlike filepath.WalkDir, directories are read concurrently
// (bounded by walkConcurrency), so fn must be safe to call from multiple
// goroutines and no visit order is guaranteed.
//
// fn may return filepath.SkipDir for a directory to prune it, or
// filepath.SkipAll to stop the entire walk; any other error aborts the walk
// and is returned. Unreadable directories are silently skipped, matching the
// tolerant callbacks this replaces. Context cancellation stops the walk and
// returns ctx.Err().
func WalkDirParallel(ctx context.Context, root string, fn func(path string, d fs.DirEntry) error) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	rootEntry := fs.FileInfoToDirEntry(info)
	if err := fn(root, rootEntry); err != nil {
		if errors.Is(err, filepath.SkipDir) || errors.Is(err, filepath.SkipAll) {
			return nil
		}
		return err
	}
	if !rootEntry.IsDir() {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(walkConcurrency)

	var walk func(dir string) error
	walk = func(dir string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		var subdirs []string
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			if err := fn(path, e); err != nil {
				if errors.Is(err, filepath.SkipDir) && e.IsDir() {
					continue
				}
				return err
			}
			if e.IsDir() {
				subdirs = append(subdirs, path)
			}
		}
		for _, sd := range subdirs {
			// TryGo hands the subdirectory to another goroutine when the
			// group has capacity; otherwise recurse inline. Blocking in
			// g.Go here could deadlock: every worker would hold its slot
			// while waiting for a free one.
			if !g.TryGo(func() error { return walk(sd) }) {
				if err := walk(sd); err != nil {
					return err
				}
			}
		}
		return nil
	}

	g.Go(func() error { return walk(root) })
	if err := g.Wait(); err != nil {
		if errors.Is(err, filepath.SkipAll) {
			return nil
		}
		return err
	}
	return nil
}
