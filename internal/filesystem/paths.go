package filesystem

import "strings"

// DirPrefix normalizes a directory path for SQL prefix matching, ensuring a
// trailing slash so "LIKE prefix%" matches children but not sibling names
// that share a textual prefix (e.g. /foo vs /foobar).
func DirPrefix(dirPath string) string {
	if dirPath != "" && !strings.HasSuffix(dirPath, "/") {
		return dirPath + "/"
	}
	return dirPath
}

// IsDirectChild reports whether relPath is a direct child under prefix (no
// further path separators after the prefix).
func IsDirectChild(relPath, prefix string) bool {
	rel := strings.TrimPrefix(relPath, prefix)
	return !strings.Contains(rel, "/")
}
