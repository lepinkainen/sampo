package classification

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
)

func TestDistanceSimilarityMapping(t *testing.T) {
	cases := []struct {
		dist int
		sim  int
	}{
		{0, 100},
		{3, 95},
		{7, 89},
		{64, 0},
	}
	for _, c := range cases {
		if got := DistanceToSimilarity(c.dist); got != c.sim {
			t.Errorf("DistanceToSimilarity(%d) = %d, want %d", c.dist, got, c.sim)
		}
	}

	thresholds := []struct {
		pct     int
		maxDist int
	}{
		{100, 0},
		{88, 7},
		{80, 12},
	}
	for _, c := range thresholds {
		if got := SimilarityToMaxDistance(c.pct); got != c.maxDist {
			t.Errorf("SimilarityToMaxDistance(%d) = %d, want %d", c.pct, got, c.maxDist)
		}
	}
}

func TestParsePHash(t *testing.T) {
	if v, ok := parsePHash("00000000000000ff"); !ok || v != 0xff {
		t.Errorf("parsePHash hex = %x, %v", v, ok)
	}
	for _, bad := range []string{"", "zz", "123", "gggggggggggggggg"} {
		if _, ok := parsePHash(bad); ok {
			t.Errorf("parsePHash(%q) should fail", bad)
		}
	}
}

func TestGroupSimilarClustering(t *testing.T) {
	entries := []PHashEntry{
		{RelPath: "a.jpg", PHash: 0x0000000000000000, Width: 400, Height: 300},
		{RelPath: "b.jpg", PHash: 0x0000000000000003, Width: 200, Height: 150}, // 2 bits from a
		{RelPath: "c.jpg", PHash: 0x0000000000000007, Width: 100, Height: 75},  // 1 bit from b, 3 from a (transitive)
		{RelPath: "d.jpg", PHash: 0xffffffffffffffff, Width: 400, Height: 300}, // far from everything
	}

	groups := groupSimilar(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	if len(groups[0]) != 3 {
		t.Fatalf("cluster size = %d, want 3 (transitive chain)", len(groups[0]))
	}

	// Threshold boundary: distance 3 between a and c only links via b.
	groups = groupSimilar(entries[:1], 2)
	if groups != nil {
		t.Fatalf("single entry should produce no groups, got %v", groups)
	}
}

func TestGroupSimilarCollapsesIdenticalSHA256(t *testing.T) {
	entries := []PHashEntry{
		{RelPath: "orig.jpg", PHash: 0x1234, SHA256: "same"},
		{RelPath: "copy.jpg", PHash: 0x1234, SHA256: "same"}, // byte-identical → exact tab
	}
	if groups := groupSimilar(entries, 4); groups != nil {
		t.Fatalf("byte-identical pair should not form a similar group, got %v", groups)
	}

	// A third, near-but-different file still groups with the representative.
	entries = append(entries, PHashEntry{RelPath: "resized.jpg", PHash: 0x1235, SHA256: "other"})
	groups := groupSimilar(entries, 4)
	if len(groups) != 1 || len(groups[0]) != 2 {
		t.Fatalf("groups = %v, want 1 group with representative + resized", groups)
	}
}

func TestBuildSimilarGroupKeeper(t *testing.T) {
	members := []PHashEntry{
		{RelPath: "small.jpg", PHash: 0x03, Width: 640, Height: 480},
		{RelPath: "big.jpg", PHash: 0x00, Width: 4032, Height: 3024},
	}
	g := buildSimilarGroup("root-0", members)
	if g.Keeper == nil || *g.Keeper != 0 {
		t.Fatalf("keeper = %v, want index 0 (highest resolution)", g.Keeper)
	}
	if g.Files[0].Path != "big.jpg" {
		t.Fatalf("files[0] = %s, want big.jpg (sorted best first)", g.Files[0].Path)
	}
	if g.Files[0].Similarity != 100 || g.Files[1].Similarity != DistanceToSimilarity(2) {
		t.Fatalf("similarities = %d, %d", g.Files[0].Similarity, g.Files[1].Similarity)
	}
	if g.MaxDistance != 2 {
		t.Fatalf("maxDistance = %d, want 2", g.MaxDistance)
	}

	// Same resolution → quality tie → no keeper.
	tie := []PHashEntry{
		{RelPath: "x.png", PHash: 0x00, Width: 3600, Height: 2400},
		{RelPath: "y.webp", PHash: 0x01, Width: 3600, Height: 2400},
	}
	g = buildSimilarGroup("root-0", tie)
	if g.Keeper != nil {
		t.Fatalf("keeper = %v, want nil on resolution tie", *g.Keeper)
	}
}

// TestComputePHashGolden pins the phash of a checked-in test image so a
// dependency upgrade that silently changes hash values (invalidating every
// stored phash) fails loudly. If this breaks after an intentional algorithm
// change, migrate by nulling the phash column and update the golden value.
func TestComputePHashGolden(t *testing.T) {
	img, err := imaging.Open(filepath.Join("testdata", "person_einstein.jpg"))
	if err != nil {
		t.Fatalf("opening test image: %v", err)
	}

	hash1, err := ComputePHash(img)
	if err != nil {
		t.Fatalf("ComputePHash: %v", err)
	}
	hash2, err := ComputePHash(img)
	if err != nil {
		t.Fatalf("ComputePHash second run: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("phash not deterministic: %s vs %s", hash1, hash2)
	}
	if len(hash1) != 16 {
		t.Fatalf("phash length = %d, want 16 hex chars (%s)", len(hash1), hash1)
	}

	golden := goldenPHash(t)
	if hash1 != golden {
		t.Fatalf("phash = %s, want golden %s (dependency drift? see comment)", hash1, golden)
	}

	// A downscaled re-encode should stay within a few bits.
	small := imaging.Resize(img, img.Bounds().Dx()/2, 0, imaging.Lanczos)
	smallHash, err := ComputePHash(small)
	if err != nil {
		t.Fatalf("ComputePHash resized: %v", err)
	}
	a, _ := parsePHash(hash1)
	b, _ := parsePHash(smallHash)
	if d := hammingDistance(a, b); d > 6 {
		t.Fatalf("resized image distance = %d bits, want ≤ 6", d)
	}
}

// goldenPHash reads the pinned hash from testdata, generating it on first
// run so the golden file records whatever the pinned dependency produces.
func goldenPHash(t *testing.T) string {
	t.Helper()
	goldenPath := filepath.Join("testdata", "person_einstein.phash.golden")
	data, err := os.ReadFile(goldenPath)
	if err == nil {
		return string(data)
	}
	img, err := imaging.Open(filepath.Join("testdata", "person_einstein.jpg"))
	if err != nil {
		t.Fatalf("opening test image for golden: %v", err)
	}
	hash, err := ComputePHash(img)
	if err != nil {
		t.Fatalf("computing golden phash: %v", err)
	}
	if err := os.WriteFile(goldenPath, []byte(hash), 0o644); err != nil {
		t.Fatalf("writing golden file: %v", err)
	}
	t.Logf("wrote golden phash %s to %s", hash, goldenPath)
	return hash
}
