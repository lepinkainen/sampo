package handlers

import (
	"sync"
	"time"

	"github.com/lepinkainen/sampo/internal/filesystem"
)

// dirListTTL is how long a cached directory listing is considered fresh.
// The slow part of a listing is the OS-level readdir (and per-entry stat
// round-trips) on network mounts; per-request enrichment (detection /
// classification / OCR / checksum / dimension lookups) runs on top of the
// cached slice every time, so tags and detections still update live. The TTL
// only bounds staleness of the file membership itself; any mutation
// (delete/move/copy/rename) invalidates the affected root immediately.
const dirListTTL = 5 * time.Second

// dirListCache memoizes filesystem.ListDirectory results for a short TTL.
type dirListCache struct {
	mu sync.RWMutex
	m  map[string]dirListCacheEntry
}

type dirListCacheEntry struct {
	entries []filesystem.FileEntry
	expires time.Time
}

func newDirListCache() *dirListCache {
	return &dirListCache{m: make(map[string]dirListCacheEntry)}
}

func dirListKey(rootID, relPath string) string {
	return rootID + ":" + relPath
}

// get returns a fresh shallow copy of the cached entries so per-request
// enrichment can mutate element fields without polluting the cache. Returns
// (nil, false) on a miss or expired entry.
func (c *dirListCache) get(rootID, relPath string) ([]filesystem.FileEntry, bool) {
	c.mu.RLock()
	e, ok := c.m[dirListKey(rootID, relPath)]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	cp := make([]filesystem.FileEntry, len(e.entries))
	copy(cp, e.entries)
	return cp, true
}

func (c *dirListCache) put(rootID, relPath string, entries []filesystem.FileEntry) {
	cp := make([]filesystem.FileEntry, len(entries))
	copy(cp, entries)

	c.mu.Lock()
	c.m[dirListKey(rootID, relPath)] = dirListCacheEntry{
		entries: cp,
		expires: time.Now().Add(dirListTTL),
	}
	c.mu.Unlock()
}

// invalidateRoot drops every cached listing for a root. Called after any
// mutation so stale directory contents don't surface within the TTL window.
func (c *dirListCache) invalidateRoot(rootID string) {
	prefix := rootID + ":"
	c.mu.Lock()
	for k := range c.m {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.m, k)
		}
	}
	c.mu.Unlock()
}
