package stash

import (
	"testing"
)

// testPerformers are the candidate performers used across test cases.
var testPerformers = []Performer{
	{Name: "Pamela Anderson", Aliases: []string{"Pam Anderson"}},
	{Name: "Rachel Cook", Aliases: []string{"Rachel"}},
	{Name: "Emma Watson", Aliases: []string{}},
}

// TestNormalize verifies the normalize helper.
func TestNormalize(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Rachel Cook beach.mp4", "rachel cook beach mp4"},
		{"001.jpg", "001 jpg"},
		{"  hello--world  ", "hello world"},
		{"", ""},
		{"UPPER CASE", "upper case"},
		{"foo_bar-baz", "foo bar baz"},
	}
	for _, c := range cases {
		got := normalize(c.input)
		if got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestStripExt verifies extension stripping.
func TestStripExt(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"001.jpg", "001"},
		{"Rachel Cook beach.mp4", "Rachel Cook beach"},
		{"noext", "noext"},
		{".hidden", ""},
	}
	for _, c := range cases {
		got := stripExt(c.input)
		if got != c.want {
			t.Errorf("stripExt(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestDirnameFallback tests that files with numeric/opaque names use the source
// directory name to match a performer. E.g. Pamela Anderson/001.jpg should match
// via dirname "Pamela Anderson".
func TestDirnameFallback(t *testing.T) {
	files := [][2]string{
		{"Pamela Anderson/001.jpg", "001.jpg"},
		{"Pamela Anderson/002.jpg", "002.jpg"},
		{"Pamela Anderson/video.mp4", "video.mp4"},
	}

	groups, unmatched := Match(testPerformers, "Pamela Anderson", files)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %d: %+v", len(unmatched), unmatched)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.Performer != "Pamela Anderson" {
		t.Errorf("expected performer Pamela Anderson, got %q", g.Performer)
	}
	if g.MatchedBy != MatchedByDirname {
		t.Errorf("expected dirname match, got %q", g.MatchedBy)
	}
	if len(g.Files) != 3 {
		t.Errorf("expected 3 files, got %d", len(g.Files))
	}
}

// TestFilenameMatch tests that a file explicitly named after a performer matches
// by filename even when the directory name is generic (a date string).
func TestFilenameMatch(t *testing.T) {
	files := [][2]string{
		{"2025-01-05/Rachel Cook beach.mp4", "Rachel Cook beach.mp4"},
	}

	groups, unmatched := Match(testPerformers, "2025-01-05", files)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %d: %+v", len(unmatched), unmatched)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.Performer != "Rachel Cook" {
		t.Errorf("expected performer Rachel Cook, got %q", g.Performer)
	}
	if g.MatchedBy != MatchedByFilename {
		t.Errorf("expected filename match, got %q", g.MatchedBy)
	}
}

// TestUnmatched tests that a file with no performer match ends up in unmatched.
func TestUnmatched(t *testing.T) {
	files := [][2]string{
		{"inbox/junk.zip", "junk.zip"},
	}

	groups, unmatched := Match(testPerformers, "inbox", files)

	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
	if len(unmatched) != 1 {
		t.Errorf("expected 1 unmatched, got %d", len(unmatched))
	}
}

// TestTokenScoreFullNameBeatsPartial tests that when multiple performers share a
// first-name token, the one with MORE matching tokens wins.
// "Rachel Cook beach.mp4" contains tokens "rachel" and "cook" — both match
// "Rachel Cook" (2 hits) but only "rachel" matches "Rachel Starr" (1 hit).
func TestTokenScoreFullNameBeatsPartial(t *testing.T) {
	performers := []Performer{
		{Name: "Rachel Cook", Aliases: []string{}},
		{Name: "Rachel Starr", Aliases: []string{}},
	}

	files := [][2]string{
		{"2025-01-05/Rachel Cook beach.mp4", "Rachel Cook beach.mp4"},
	}

	groups, unmatched := Match(performers, "2025-01-05", files)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %v", unmatched)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Performer != "Rachel Cook" {
		t.Errorf("full-name token score should win: expected Rachel Cook, got %q", groups[0].Performer)
	}
}

// TestAliasMatch tests that a performer is matched via an alias in the filename.
// Both tokens of the alias "Pam Anderson" must be found as whole tokens.
func TestAliasMatch(t *testing.T) {
	performers := []Performer{
		{Name: "Pamela Anderson", Aliases: []string{"Pam Anderson", "Pammy"}},
	}

	files := [][2]string{
		{"inbox/Pam Anderson pool shoot.jpg", "Pam Anderson pool shoot.jpg"},
	}

	groups, unmatched := Match(performers, "inbox", files)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %v", unmatched)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Performer != "Pamela Anderson" {
		t.Errorf("alias should match performer: got %q", groups[0].Performer)
	}
	if groups[0].MatchedBy != MatchedByFilename {
		t.Errorf("alias matched via filename: expected filename, got %q", groups[0].MatchedBy)
	}
}

// TestNoArchiveDirStillMatches tests that a performer WITHOUT an existing archive
// directory is now still matched (the old existingDirs filter is gone).
func TestNoArchiveDirStillMatches(t *testing.T) {
	// Emma Watson exists in performers but has no existing archive dir — she
	// should still be matched now that the filter is removed.
	files := [][2]string{
		{"inbox/Emma Watson red carpet.jpg", "Emma Watson red carpet.jpg"},
	}

	groups, unmatched := Match(testPerformers, "inbox", files)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched (Emma Watson should match without an archive dir), got %d", len(unmatched))
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Performer != "Emma Watson" {
		t.Errorf("expected Emma Watson, got %q", groups[0].Performer)
	}
	// TargetPath should be the performer name (no existing dir to resolve to).
	if groups[0].TargetPath != "Emma Watson" {
		t.Errorf("expected TargetPath %q, got %q", "Emma Watson", groups[0].TargetPath)
	}
}

// TestRegressionBrianaVsBrianna is the regression test for the substring-match bug.
// Performers: "Briana Nicole" (alias "briinic") and "Brianna Torres" (aliases "Bri", "missbrisolo").
// File "34431857-Briana-07733.jpg" → normalized text "34431857 briana 07733".
// "bri" is a 3-char substring of "briana" but NOT a whole token — so Brianna Torres must NOT match.
// "briana" IS a whole token, and it is a whole token in "briana nicole" → Briana Nicole matches.
func TestRegressionBrianaVsBrianna(t *testing.T) {
	performers := []Performer{
		{Name: "Briana Nicole", Aliases: []string{"briinic"}},
		{Name: "Brianna Torres", Aliases: []string{"Bri", "missbrisolo"}},
	}

	// "34431857-Briana-07733.jpg" → stripExt → "34431857-Briana-07733" → normalize → "34431857 briana 07733"
	files := [][2]string{
		{"inbox/34431857-Briana-07733.jpg", "34431857-Briana-07733.jpg"},
	}

	groups, unmatched := Match(performers, "inbox", files)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %d: %+v", len(unmatched), unmatched)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Performer != "Briana Nicole" {
		t.Errorf("regression: expected Briana Nicole (whole-token 'briana'), got %q", groups[0].Performer)
	}
}

// TestAliasWholeTokenOnly asserts that a short alias like "Bri" does NOT match
// a file containing the token "briana" — the alias token must match whole.
// A file "briana.jpg" should match "Briana Nicole" (via token "briana" in its name),
// NOT "Brianna Torres" (whose alias "bri" is NOT the token "briana").
func TestAliasWholeTokenOnly(t *testing.T) {
	performers := []Performer{
		{Name: "Briana Nicole", Aliases: []string{"briinic"}},
		{Name: "Brianna Torres", Aliases: []string{"Bri", "missbrisolo"}},
	}

	// "briana.jpg" → stripExt → "briana" → normalize → "briana"
	files := [][2]string{
		{"inbox/briana.jpg", "briana.jpg"},
	}

	groups, unmatched := Match(performers, "inbox", files)

	if len(unmatched) != 0 {
		t.Errorf("expected 0 unmatched, got %d: %+v", len(unmatched), unmatched)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	// "briana" whole-token matches "briana" in "briana nicole" (1 hit).
	// "bri" is NOT the whole token "briana", so Brianna Torres gets 0 hits.
	if groups[0].Performer != "Briana Nicole" {
		t.Errorf("whole-token alias: expected Briana Nicole, got %q", groups[0].Performer)
	}
}
