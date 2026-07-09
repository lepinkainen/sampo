package archive

import (
	"archive/zip"
	"fmt"
	"io"
)

// listZipEntries lists all entries (files and directories) in a zip archive,
// scanning at most maxEntries entries.
func listZipEntries(p string) ([]Entry, error) {
	rc, err := zip.OpenReader(p)
	if err != nil {
		return nil, fmt.Errorf("archive: opening zip %s: %w", p, err)
	}
	defer func() { _ = rc.Close() }()

	entries := make([]Entry, 0, len(rc.File))
	for i, f := range rc.File {
		if i >= maxEntries {
			break
		}
		entries = append(entries, Entry{
			Name: normalizeName(f.Name),
			Size: int64FromUint64(f.UncompressedSize64),
		})
	}
	return entries, nil
}

// readZipCover extracts the cover image from a zip/cbz archive: it collects
// all candidate image entries, then picks the lexicographically-first name
// (see selectCoverName), skipping any candidate whose uncompressed size
// exceeds maxCoverBytes in favor of the next one in sorted order.
func readZipCover(p string) (data []byte, name string, err error) {
	rc, err := zip.OpenReader(p)
	if err != nil {
		return nil, "", fmt.Errorf("archive: opening zip %s: %w", p, err)
	}
	defer func() { _ = rc.Close() }()

	candidatesByName := make(map[string]*zip.File)
	candidateNames := make([]string, 0)

	for i, f := range rc.File {
		if i >= maxEntries {
			break
		}
		normalized := normalizeName(f.Name)
		if !isCoverCandidate(normalized, f.FileInfo().IsDir()) {
			continue
		}
		candidatesByName[normalized] = f
		candidateNames = append(candidateNames, normalized)
	}

	if len(candidateNames) == 0 {
		return nil, "", ErrNoImages
	}

	for len(candidateNames) > 0 {
		chosen := selectCoverName(candidateNames)
		if chosen == "" {
			return nil, "", ErrNoImages
		}
		f := candidatesByName[chosen]
		if f.UncompressedSize64 > maxCoverBytes {
			// Too large: drop this candidate and try the next in sorted order.
			candidateNames = removeName(candidateNames, chosen)
			delete(candidatesByName, chosen)
			continue
		}

		data, err := readZipFile(f)
		if err != nil {
			return nil, "", err
		}
		return data, chosen, nil
	}

	return nil, "", ErrNoImages
}

// readZipFile reads the full contents of a zip entry, enforcing
// maxCoverBytes as a hard cap (rejecting rather than silently truncating a
// mismatched/lying size header).
func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("archive: opening zip entry %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(io.LimitReader(rc, maxCoverBytes+1))
	if err != nil {
		return nil, fmt.Errorf("archive: reading zip entry %s: %w", f.Name, err)
	}
	if len(data) > maxCoverBytes {
		return nil, fmt.Errorf("archive: zip entry %s exceeds max cover size", f.Name)
	}
	return data, nil
}

// removeName returns names with the given value removed (first occurrence).
func removeName(names []string, value string) []string {
	out := make([]string, 0, len(names))
	removed := false
	for _, n := range names {
		if !removed && n == value {
			removed = true
			continue
		}
		out = append(out, n)
	}
	return out
}
