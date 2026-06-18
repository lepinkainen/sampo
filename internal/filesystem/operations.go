package filesystem

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// StatPath returns os.FileInfo for a path.
func StatPath(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// Delete removes a file or directory. If recursive is true, non-empty directories are removed.
func Delete(path string, recursive bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if info.IsDir() && !recursive {
		// Check if directory is empty
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return fmt.Errorf("directory is not empty (use recursive=true)")
		}
	}

	if info.IsDir() && recursive {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// FileChecksum computes SHA-256 of a file via streaming.
func FileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// autoRename returns a non-conflicting path by appending (2), (3), etc.
func autoRename(dst string) string {
	ext := filepath.Ext(dst)
	base := strings.TrimSuffix(dst, ext)

	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// CopyFile copies a single file from src to dst. If dst exists, uses smart dedup:
// matching checksums = skip (identical), different = auto-rename.
// Returns the actual destination path used.
func CopyFile(src, dst string) (string, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if srcInfo.IsDir() {
		return "", fmt.Errorf("source is a directory, use CopyDir")
	}

	dst, err = resolveConflict(src, dst)
	if errors.Is(err, errIdentical) {
		return dst, nil // no copy needed
	}
	if err != nil {
		return "", err
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkdirErr != nil {
		return "", mkdirErr
	}

	return dst, copyFileData(src, dst, srcInfo.Mode())
}

// resolveConflict checks if dst exists and handles dedup/rename.
// Returns the (possibly renamed) destination path, or "" with nil error if files are identical.
func resolveConflict(src, dst string) (string, error) {
	if _, statErr := os.Stat(dst); statErr != nil {
		return dst, nil // no conflict
	}

	srcSum, err := FileChecksum(src)
	if err != nil {
		return "", fmt.Errorf("checksumming source: %w", err)
	}
	dstSum, err := FileChecksum(dst)
	if err != nil {
		return "", fmt.Errorf("checksumming destination: %w", err)
	}
	if srcSum == dstSum {
		return dst, errIdentical
	}
	return autoRename(dst), nil
}

var errIdentical = errors.New("files are identical")

func copyFileData(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, cpErr := io.Copy(out, in); cpErr != nil {
		_ = out.Close()
		return cpErr
	}
	return out.Close()
}

// CopyDir recursively copies a directory from src to dst.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if mkErr := os.MkdirAll(dst, srcInfo.Mode()); mkErr != nil {
		return mkErr
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if cpErr := CopyDir(srcPath, dstPath); cpErr != nil {
				return cpErr
			}
		} else {
			if _, cpErr := CopyFile(srcPath, dstPath); cpErr != nil {
				return cpErr
			}
		}
	}
	return nil
}

// MoveFile moves a file or directory. Tries os.Rename first (same filesystem),
// falls back to copy+delete for cross-filesystem moves.
// Returns the actual destination path used (may differ if auto-renamed).
func MoveFile(src, dst string) (string, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", err
	}

	// Handle conflict for files
	if !srcInfo.IsDir() {
		dst, err = resolveConflict(src, dst)
		if errors.Is(err, errIdentical) {
			// Identical — just delete source
			return dst, os.Remove(src)
		}
		if err != nil {
			return "", err
		}
	}

	if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
		return "", mkErr
	}

	// Try rename first (fast, same filesystem)
	if renameErr := os.Rename(src, dst); renameErr == nil {
		return dst, nil
	}

	// Cross-filesystem: copy + delete
	if srcInfo.IsDir() {
		if cpErr := CopyDir(src, dst); cpErr != nil {
			return "", cpErr
		}
	} else {
		if _, cpErr := CopyFile(src, dst); cpErr != nil {
			return "", cpErr
		}
	}

	return dst, os.RemoveAll(src)
}
