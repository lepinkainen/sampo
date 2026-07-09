package stash

import (
	"path/filepath"
	"strings"
	"unicode"
)

// MatchedBy indicates whether a file was matched by its own filename or its parent directory name.
type MatchedBy string

const (
	// MatchedByFilename means the performer was identified from the file's own name.
	MatchedByFilename MatchedBy = "filename"
	// MatchedByDirname means the performer was identified from the source directory name.
	MatchedByDirname MatchedBy = "dirname"
)

// FileMatch associates a source file path+name with the performer it matched.
type FileMatch struct {
	Path      string    // relative to source root
	Name      string    // basename
	MatchedBy MatchedBy // how the match was found
}

// MatchGroup is all files that should move to a given performer directory.
type MatchGroup struct {
	Performer  string
	TargetPath string // relative path under the archive root (just the performer dir name)
	MatchedBy  MatchedBy
	Files      []FileMatch
}

// normalize lowercases s, replaces every run of non-alphanumeric characters with
// a single space, and trims leading/trailing whitespace.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inSep := true // start true so leading separators are trimmed
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			inSep = false
		} else {
			if !inSep {
				b.WriteByte(' ')
			}
			inSep = true
		}
	}
	// Trim trailing space that may have been written for a trailing separator.
	result := b.String()
	return strings.TrimRight(result, " ")
}

// stripExt removes the file extension from a filename.
func stripExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return name
	}
	return name[:len(name)-len(ext)]
}

// bestMatch returns the performer name that best matches the normalized text,
// or an empty string if there is no match.
//
// Matching uses whole-token overlap scoring to avoid false positives from
// substring matches (e.g. alias "Bri" must not match token "briana").
//
// Algorithm:
//  1. Tokenize text into a set of whole tokens (strings.Fields on already-normalized text).
//  2. For each performer, consider needle phrases = performer Name + each Alias.
//     For each phrase, tokenize it and count hits = tokens with len >= 3 that appear
//     in the text token set (whole-token equality). Track hitLen = sum of hit token lengths.
//     Take the performer's best phrase (most hits; tie → larger hitLen).
//  3. A performer is a candidate only if its best phrase has hits >= 1.
//  4. Return the performer with the highest (hits, hitLen) score; on ties keep the
//     first-encountered (strict-greater replacement only).
func bestMatch(text string, performers []Performer) string {
	// Build token set from the already-normalized text.
	textTokens := make(map[string]bool)
	for _, tok := range strings.Fields(text) {
		textTokens[tok] = true
	}

	type score struct {
		hits   int
		hitLen int
	}

	var bestPerformer string
	var best score

	for _, p := range performers {
		// Collect normalized needle phrases: performer name + each alias.
		phrases := make([]string, 0, 1+len(p.Aliases))
		phrases = append(phrases, normalize(p.Name))
		for _, a := range p.Aliases {
			if n := normalize(a); n != "" {
				phrases = append(phrases, n)
			}
		}

		// Find the best-scoring phrase for this performer.
		var pBest score
		for _, phrase := range phrases {
			if phrase == "" {
				continue
			}
			var s score
			for _, tok := range strings.Fields(phrase) {
				if len(tok) >= 3 && textTokens[tok] {
					s.hits++
					s.hitLen += len(tok)
				}
			}
			if s.hits > pBest.hits || (s.hits == pBest.hits && s.hitLen > pBest.hitLen) {
				pBest = s
			}
		}

		// A performer is a candidate only if at least one token matched.
		if pBest.hits < 1 {
			continue
		}

		// Strictly-greater replacement keeps first-encountered on ties.
		if pBest.hits > best.hits || (pBest.hits == best.hits && pBest.hitLen > best.hitLen) {
			best = pBest
			bestPerformer = p.Name
		}
	}
	return bestPerformer
}

// Match groups source files by their best-matching performer.
// All performers are candidates regardless of whether an archive directory exists.
// The TargetPath on each group is set to the performer name; the caller (handler)
// is responsible for overriding it with the exact-case existing directory name if
// one is found, and for setting the Exists field.
//
//   - performers: full list from Stash
//   - sourceDirName: the basename of the source directory being organized (used for dirname fallback)
//   - files: list of (relPath, basename) pairs to match
func Match(
	performers []Performer,
	sourceDirName string,
	files [][2]string, // [relPath, basename]
) (groups []MatchGroup, unmatched []FileMatch) {
	normalizedDir := normalize(sourceDirName)

	// Map performer name → group index for accumulation.
	groupIndex := make(map[string]int)

	for _, f := range files {
		relPath, name := f[0], f[1]

		// Try filename match first.
		normalizedFile := normalize(stripExt(name))
		performer := bestMatch(normalizedFile, performers)

		matchedBy := MatchedByFilename
		if performer == "" && normalizedDir != "" {
			// Fall back to dirname match.
			performer = bestMatch(normalizedDir, performers)
			matchedBy = MatchedByDirname
		}

		fm := FileMatch{Path: relPath, Name: name, MatchedBy: matchedBy}

		if performer == "" {
			unmatched = append(unmatched, fm)
			continue
		}

		idx, ok := groupIndex[performer]
		if !ok {
			idx = len(groups)
			groupIndex[performer] = idx
			groups = append(groups, MatchGroup{
				Performer:  performer,
				TargetPath: performer,
				MatchedBy:  matchedBy,
			})
		}
		groups[idx].Files = append(groups[idx].Files, fm)
	}

	return groups, unmatched
}
