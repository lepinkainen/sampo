package thumbnail

import (
	"errors"

	"github.com/lepinkainen/filemanager/internal/filesystem"
)

// ErrNoImages is returned when a directory contains no image files.
var ErrNoImages = errors.New("no image files in directory")

// FindFirstCachedOrGenerate returns the JPEG path for a directory thumbnail.
// It first looks for an already-cached thumbnail (iterating entries in order),
// then generates one for the first entry if nothing is cached.
func FindFirstCachedOrGenerate(entries []filesystem.ImageEntry, rootID string, cache *Cache) (string, error) {
	if len(entries) == 0 {
		return "", ErrNoImages
	}

	// Phase 1: return first entry that already has a cached thumbnail.
	for _, e := range entries {
		key := CacheKey(rootID, e.RelPath, e.ModTime, e.Size)
		if cachedPath, ok := cache.Get(rootID, key); ok {
			return cachedPath, nil
		}
	}

	// Phase 2: generate thumbnail for the first image.
	first := entries[0]
	key := CacheKey(rootID, first.RelPath, first.ModTime, first.Size)
	if err := cache.EnsureDir(rootID); err != nil {
		return "", err
	}
	dstPath := cache.Path(rootID, key)
	if err := GenerateImageThumbnail(first.AbsPath, dstPath); err != nil {
		return "", err
	}
	return dstPath, nil
}
