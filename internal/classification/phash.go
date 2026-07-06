package classification

import (
	"fmt"
	"image"
	"math/bits"
	"sort"
	"strconv"

	"github.com/corona10/goimagehash"
)

// ComputePHash returns the 64-bit DCT perceptual hash of an image as a
// 16-char hex string (matching the sha256/crc32 columns' text encoding).
// The goimagehash dependency is pinned: hashes persist in the database, so
// upgrading across a release that changes hash values requires nulling the
// phash column to force a backfill.
func ComputePHash(img image.Image) (string, error) {
	h, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return "", fmt.Errorf("computing phash: %w", err)
	}
	return fmt.Sprintf("%016x", h.GetHash()), nil
}

// parsePHash decodes a stored 16-char hex phash into its uint64 form.
func parsePHash(hexStr string) (uint64, bool) {
	if len(hexStr) != 16 {
		return 0, false
	}
	v, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// hammingDistance counts differing bits between two 64-bit hashes.
func hammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// DistanceToSimilarity maps a Hamming distance (0..64) to a similarity percentage.
func DistanceToSimilarity(d int) int {
	return (100*(64-d) + 32) / 64
}

// SimilarityToMaxDistance maps a similarity percentage to the maximum Hamming
// distance that still counts as a match (88% → 7 bits).
func SimilarityToMaxDistance(pct int) int {
	return 64 * (100 - pct) / 100
}

// PHashEntry is one candidate file fed into similar-image clustering.
type PHashEntry struct {
	RelPath string
	PHash   uint64
	Size    int64
	Width   int
	Height  int
	Mtime   int64
	SHA256  string
}

// groupSimilar clusters entries whose perceptual hashes are within maxDist
// bits of each other (transitively, via union-find). Byte-identical files
// (same SHA256) are collapsed to a single representative first — identical
// sets belong to the exact-duplicates view, and without the collapse every
// exact group would reappear as a zero-distance similar group. Clusters with
// fewer than two members after collapsing are dropped.
func groupSimilar(entries []PHashEntry, maxDist int) [][]PHashEntry {
	// Collapse identical SHA256s (empty SHA256 means unknown; keep those).
	seen := make(map[string]bool)
	var cands []PHashEntry
	for _, e := range entries {
		if e.SHA256 != "" {
			if seen[e.SHA256] {
				continue
			}
			seen[e.SHA256] = true
		}
		cands = append(cands, e)
	}

	n := len(cands)
	if n < 2 {
		return nil
	}

	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// O(n²) pairwise popcount; fine for directory-scoped scans (≤ ~20k
	// images is well under a second). A BK-tree is the upgrade path if a
	// library-wide view lands.
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if hammingDistance(cands[i].PHash, cands[j].PHash) <= maxDist {
				union(i, j)
			}
		}
	}

	clusters := make(map[int][]PHashEntry)
	for i, e := range cands {
		root := find(i)
		clusters[root] = append(clusters[root], e)
	}

	var roots []int
	for root, members := range clusters {
		if len(members) >= 2 {
			roots = append(roots, root)
		}
	}
	sort.Ints(roots)

	groups := make([][]PHashEntry, 0, len(roots))
	for _, root := range roots {
		groups = append(groups, clusters[root])
	}
	return groups
}
