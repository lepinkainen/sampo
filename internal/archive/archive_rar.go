package archive

import (
	"errors"
	"fmt"
	"io"

	rardecode "github.com/nwaples/rardecode/v2"
)

// listRarEntries lists all entries (files and directories) in a rar archive,
// scanning at most maxEntries entries. Only headers are read, not content.
func listRarEntries(p string) ([]Entry, error) {
	rc, err := rardecode.OpenReader(p)
	if err != nil {
		return nil, wrapRarOpenErr(p, err)
	}
	defer func() { _ = rc.Close() }()

	entries := make([]Entry, 0)
	for range maxEntries {
		h, err := rc.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, wrapRarNextErr(p, err)
		}
		entries = append(entries, Entry{
			Name: normalizeName(h.Name),
			Size: h.UnPackedSize,
		})
	}
	return entries, nil
}

// readRarCover extracts the cover image from a rar/cbr archive. RAR is a
// strictly streaming format — the reader can't peek ahead or cheaply seek
// back to compare candidates the way the zip path does — so the first
// matching image entry in stream order wins (no cross-entry sort). A
// two-pass approach would double the I/O cost, and CBR pages are
// conventionally stored in reading order anyway, so stream-order matches
// sorted-order closely enough in practice for comics.
func readRarCover(p string) (data []byte, name string, err error) {
	rc, err := rardecode.OpenReader(p)
	if err != nil {
		return nil, "", wrapRarOpenErr(p, err)
	}
	defer func() { _ = rc.Close() }()

	for range maxEntries {
		h, err := rc.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", wrapRarNextErr(p, err)
		}

		normalized := normalizeName(h.Name)
		if h.IsDir || !isCoverCandidate(normalized, h.IsDir) {
			continue
		}
		if !h.UnKnownSize && h.UnPackedSize > maxCoverBytes {
			// Too large: skip this entry and keep scanning in stream order.
			continue
		}

		buf, err := io.ReadAll(io.LimitReader(&rc.Reader, maxCoverBytes+1))
		if err != nil {
			return nil, "", fmt.Errorf("archive: reading rar entry %s: %w", normalized, err)
		}
		if len(buf) > maxCoverBytes {
			return nil, "", fmt.Errorf("archive: rar entry %s exceeds max cover size", normalized)
		}
		return buf, normalized, nil
	}

	return nil, "", ErrNoImages
}

// wrapRarOpenErr maps rardecode's encryption sentinels to our own
// ErrEncrypted and wraps any other error as a corrupt-archive error.
func wrapRarOpenErr(p string, err error) error {
	if errors.Is(err, rardecode.ErrArchiveEncrypted) || errors.Is(err, rardecode.ErrArchivedFileEncrypted) {
		return fmt.Errorf("archive: opening rar %s: %w", p, ErrEncrypted)
	}
	return fmt.Errorf("archive: opening rar %s: %w", p, err)
}

// wrapRarNextErr maps rardecode's encryption sentinels (which can also
// surface while advancing through entries) to ErrEncrypted, wrapping any
// other error as a corrupt-archive error.
func wrapRarNextErr(p string, err error) error {
	if errors.Is(err, rardecode.ErrArchiveEncrypted) || errors.Is(err, rardecode.ErrArchivedFileEncrypted) {
		return fmt.Errorf("archive: reading rar %s: %w", p, ErrEncrypted)
	}
	return fmt.Errorf("archive: reading rar %s: %w", p, err)
}
