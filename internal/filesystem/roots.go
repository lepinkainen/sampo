package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lepinkainen/filemanager/internal/config"
)

// Root represents a mounted directory root.
type Root struct {
	ID   string
	Name string
	Path string // absolute, cleaned path
}

// RootManager manages multiple filesystem roots.
type RootManager struct {
	mu    sync.RWMutex
	roots map[string]*Root
	order []string // preserve config order
}

// NewRootManager creates a RootManager from config.
func NewRootManager(roots []config.RootConfig) (*RootManager, error) {
	rm := &RootManager{
		roots: make(map[string]*Root),
	}

	for i, rc := range roots {
		absPath, err := filepath.Abs(rc.Path)
		if err != nil {
			return nil, fmt.Errorf("resolving path for root %q: %w", rc.Name, err)
		}

		// Resolve symlinks to get canonical path for reliable prefix checking
		absPath, err = filepath.EvalSymlinks(absPath)
		if err != nil {
			return nil, fmt.Errorf("resolving symlinks for root %q: %w", rc.Name, err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("root %q path %q: %w", rc.Name, absPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("root %q path %q is not a directory", rc.Name, absPath)
		}

		id := fmt.Sprintf("root-%d", i)
		rm.roots[id] = &Root{
			ID:   id,
			Name: rc.Name,
			Path: absPath,
		}
		rm.order = append(rm.order, id)
	}

	return rm, nil
}

// List returns all roots in config order.
func (rm *RootManager) List() []*Root {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*Root, 0, len(rm.order))
	for _, id := range rm.order {
		result = append(result, rm.roots[id])
	}
	return result
}

// Get returns a root by ID.
func (rm *RootManager) Get(id string) (*Root, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	root, ok := rm.roots[id]
	if !ok {
		return nil, fmt.Errorf("root %q not found", id)
	}
	return root, nil
}

// ResolvePath safely resolves a relative path within a root,
// preventing path traversal attacks.
func (rm *RootManager) ResolvePath(rootID, relPath string) (string, error) {
	root, err := rm.Get(rootID)
	if err != nil {
		return "", err
	}

	// Clean the relative path to remove traversal attempts
	cleaned := filepath.Clean("/" + relPath)
	fullPath := filepath.Join(root.Path, cleaned)

	// First check: cleaned path must still be under root
	// This catches ../.. even when the target doesn't exist
	if !strings.HasPrefix(fullPath, root.Path+string(filepath.Separator)) && fullPath != root.Path {
		return "", fmt.Errorf("path traversal detected")
	}

	// Second check: if the path exists, resolve symlinks and verify again
	resolved, err := filepath.EvalSymlinks(fullPath)
	if err == nil && !strings.HasPrefix(resolved, root.Path) {
		return "", fmt.Errorf("path traversal detected via symlink")
	}

	return fullPath, nil
}
