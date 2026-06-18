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
	OCRText   string     `json:"ocrText,omitempty"`
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

// ImageEntry holds metadata for an image file within a directory.
type ImageEntry struct {
	AbsPath string
	RelPath string
	ModTime int64
	Size    int64
}

// isVisibleImage reports whether a directory entry is a non-hidden image file.
func isVisibleImage(e os.DirEntry) bool {
	return !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && imageExts[strings.ToLower(filepath.Ext(e.Name()))]
}

// HasImageFile reports whether dirPath contains at least one image file.
func HasImageFile(dirPath string) bool {
	f, err := os.Open(dirPath)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	for {
		entries, err := f.ReadDir(64)
		for _, e := range entries {
			if isVisibleImage(e) {
				return true
			}
		}
		if err != nil {
			return false
		}
	}
}

// ImageFilesInDir returns metadata for all image files in dirPath, sorted lexicographically.
func ImageFilesInDir(dirPath, relBase string) ([]ImageEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var result []ImageEntry
	for _, e := range entries {
		if !isVisibleImage(e) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, ImageEntry{
			AbsPath: filepath.Join(dirPath, e.Name()),
			RelPath: filepath.Join(relBase, e.Name()),
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
		})
	}
	return result, nil
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

		hasThumb := mediaType == "image" || mediaType == "video"
		if e.IsDir() {
			hasThumb = HasImageFile(filepath.Join(dirPath, e.Name()))
		}

		entry := FileEntry{
			Name:      e.Name(),
			Path:      relPath,
			IsDir:     e.IsDir(),
			IsZip:     archiveExts[strings.ToLower(filepath.Ext(e.Name()))],
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			MediaType: mediaType,
			HasThumb:  hasThumb,
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
