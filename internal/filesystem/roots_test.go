package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lepinkainen/filemanager/internal/config"
)

func TestNewRootManager(t *testing.T) {
	dir := t.TempDir()

	roots := []config.RootConfig{
		{Name: "Test", Path: dir},
	}

	rm, err := NewRootManager(roots)
	if err != nil {
		t.Fatalf("NewRootManager failed: %v", err)
	}

	list := rm.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 root, got %d", len(list))
	}
	if list[0].Name != "Test" {
		t.Errorf("expected name Test, got %s", list[0].Name)
	}
}

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	os.MkdirAll(subdir, 0o755)

	roots := []config.RootConfig{
		{Name: "Test", Path: dir},
	}

	rm, err := NewRootManager(roots)
	if err != nil {
		t.Fatalf("NewRootManager failed: %v", err)
	}

	// Valid path
	resolved, err := rm.ResolvePath("root-0", "sub")
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}
	if filepath.Base(resolved) != "sub" {
		t.Errorf("expected path ending in 'sub', got %s", resolved)
	}

	// Path traversal is safely prevented by filepath.Clean rooting
	// ../../etc/passwd becomes /etc/passwd which joins under root
	traversalPath, err := rm.ResolvePath("root-0", "../../etc/passwd")
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}
	// Should resolve to {root}/etc/passwd, not /etc/passwd
	rootList := rm.List()
	if !strings.HasPrefix(traversalPath, rootList[0].Path) {
		t.Errorf("path traversal not contained: %s", traversalPath)
	}

	// Invalid root
	_, err = rm.ResolvePath("invalid", "sub")
	if err == nil {
		t.Error("expected error for invalid root, got nil")
	}
}
