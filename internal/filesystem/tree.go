package filesystem

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TagScore holds a classification tag and its confidence score.
type TagScore struct {
	Label string  `json:"label"`
	Score float32 `json:"score"`
}

// FileEntry represents a file or directory in a listing.
type FileEntry struct {
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	IsDir     bool       `json:"isDir"`
	IsZip     bool       `json:"isZip"`
	Size      int64      `json:"size"`
	ModTime   time.Time  `json:"modTime"`
	MediaType string     `json:"mediaType"`
	HasThumb  bool       `json:"hasThumb"`
	HasPerson *bool      `json:"hasPerson,omitempty"`
	Tags      []TagScore `json:"tags,omitempty"`
	SHA256    *string    `json:"sha256,omitempty"`
	CRC32     *string    `json:"crc32,omitempty"`
}

// videotaggerCRC32Re matches the CRC32 bracket group in videotagger-style filenames.
// Pattern: name_[resolution][duration][CRC32_HEX].ext
var videotaggerCRC32Re = regexp.MustCompile(`\[[A-Fa-f0-9]{8}\]`)

// ParseCRC32FromFilename extracts CRC32 from videotagger-style filenames.
// Returns empty string if not found.
func ParseCRC32FromFilename(name string) string {
	matches := videotaggerCRC32Re.FindAllString(name, -1)
	if len(matches) == 0 {
		return ""
	}
	// Take the last match (CRC32 is typically the last bracket group)
	last := matches[len(matches)-1]
	return strings.ToUpper(last[1 : len(last)-1])
}

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".webp": true, ".avif": true, ".bmp": true, ".tiff": true,
}

var videoExts = map[string]bool{
	".mp4": true, ".webm": true, ".mkv": true, ".avi": true,
	".mov": true, ".wmv": true, ".flv": true,
}

var archiveExts = map[string]bool{
	".zip": true,
}

// DetectMediaType returns the media type based on file extension.
func DetectMediaType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case imageExts[ext]:
		return "image"
	case videoExts[ext]:
		return "video"
	case archiveExts[ext]:
		return "archive"
	default:
		return "other"
	}
}

// ListDirectory returns the contents of a directory.
func ListDirectory(dirPath, relBase string) ([]FileEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	result := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		// Skip hidden files
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		relPath := filepath.Join(relBase, e.Name())
		mediaType := "other"
		if !e.IsDir() {
			mediaType = DetectMediaType(e.Name())
		}

		entry := FileEntry{
			Name:      e.Name(),
			Path:      relPath,
			IsDir:     e.IsDir(),
			IsZip:     archiveExts[strings.ToLower(filepath.Ext(e.Name()))],
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			MediaType: mediaType,
			HasThumb:  mediaType == "image" || mediaType == "video",
		}
		// Parse CRC32 from videotagger-style filenames for videos
		if mediaType == "video" {
			if crc := ParseCRC32FromFilename(e.Name()); crc != "" {
				entry.CRC32 = &crc
			}
		}
		result = append(result, entry)
	}

	return result, nil
}
