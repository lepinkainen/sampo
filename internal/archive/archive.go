// Package archive extracts a representative "cover" image from comic/image
// archives (zip/cbz, rar/cbr) for thumbnailing, plus a general entry-listing
// primitive for future browse-inside-archive use.
package archive

import (
	"errors"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/lepinkainen/sampo/internal/filesystem"
)

// maxEntries caps how many entries are scanned per archive, guarding against
// zip/rar bombs with an enormous number of entries.
const maxEntries = 10_000

// maxCoverBytes caps the uncompressed size of the selected cover entry,
// guarding against decompression bombs disguised as a single huge image.
const maxCoverBytes = 64 << 20

// ErrNoImages is returned when an archive contains no usable image entry.
var ErrNoImages = errors.New("archive: no image entries")

// ErrEncrypted is returned when an archive (or its entries) requires a
// password to read.
var ErrEncrypted = errors.New("archive: encrypted")

// Entry describes one file entry in an archive (exported for a future
// browse-inside-archive feature; not yet used outside this package).
type Entry struct {
	Name string
	Size int64
}

// ListEntries returns all entries in the archive at path (zip/cbz or
// rar/cbr, dispatched by extension), for future browse-inside use and tests.
func ListEntries(p string) ([]Entry, error) {
	switch strings.ToLower(path.Ext(p)) {
	case ".zip", ".cbz":
		return listZipEntries(p)
	case ".rar", ".cbr":
		return listRarEntries(p)
	default:
		return nil, fmt.Errorf("archive: unsupported extension %q", path.Ext(p))
	}
}

// ReadCoverImage extracts the representative "cover" image from the archive
// at path (comic-cover convention). Returns the image bytes and the entry
// name the bytes came from.
func ReadCoverImage(p string) (data []byte, name string, err error) {
	switch strings.ToLower(path.Ext(p)) {
	case ".zip", ".cbz":
		return readZipCover(p)
	case ".rar", ".cbr":
		return readRarCover(p)
	default:
		return nil, "", fmt.Errorf("archive: unsupported extension %q", path.Ext(p))
	}
}

// normalizeName converts legacy backslash separators (seen in some old zips)
// to forward slashes, so filtering and comparison logic can assume '/'.
func normalizeName(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

// isCoverCandidate reports whether name (already normalized) is a plausible
// cover image entry: not a directory, has a recognized (non-avif, since we
// have no avif decoder) image extension, is not a dotfile, and is not under
// a __MACOSX/ resource-fork directory.
func isCoverCandidate(name string, isDir bool) bool {
	if isDir {
		return false
	}
	if !filesystem.HasImageExt(name) {
		return false
	}
	if strings.ToLower(path.Ext(name)) == ".avif" {
		return false
	}
	if strings.HasPrefix(path.Base(name), ".") {
		return false
	}
	if strings.HasPrefix(name, "__MACOSX/") {
		return false
	}
	return true
}

// int64FromUint64 safely converts a size value reported as uint64 (as zip's
// UncompressedSize64 is) into int64 for Entry.Size, clamping to
// math.MaxInt64 rather than risking a silent overflow/wraparound.
func int64FromUint64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

// selectCoverName picks the cover entry name from a set of candidate names
// using sorted (lexicographic) order. Returns "" if names is empty. Used by
// the zip path (which can see all candidates up front from the central
// directory) so future per-entry thumbnails reuse identical ordering.
func selectCoverName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)
	return sorted[0]
}
