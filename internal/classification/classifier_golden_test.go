package classification

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/lepinkainen/sampo/internal/onnxenv"
)

// Golden classification fixtures. Each image lives in testdata/ and carries a
// permissive license (public domain / CC0 — see testdata/SOURCES.md). want is a
// tag that MUST be present in the result; reject tags that MUST NOT appear.
//
// This is a regression guard for the "everything tagged animal" bug: CLIP raw
// cosines cluster near ~0.2, so a flat threshold let noise labels leak. With
// per-group softmax scoring a single-person photo must surface "solo", not "animal".
var goldenCases = []struct {
	file   string
	want   string
	reject []string
}{
	{file: "person_lincoln.jpg", want: "solo", reject: []string{"animal", "vehicle"}},
	{file: "person_einstein.jpg", want: "solo", reject: []string{"animal", "vehicle"}},
	{file: "animal_eagle.jpg", want: "animal", reject: []string{"person", "vehicle"}},
	{file: "vehicle_car.jpg", want: "vehicle", reject: []string{"person", "animal"}},
}

func TestClassifyGolden(t *testing.T) {
	const (
		modelPath  = "../../models/clip-vit-b32-image.onnx"
		labelsPath = "../../models/clip-labels.json"
		threshold  = 0.2
	)

	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("CLIP model not present (%s) — run `task download-clip-model`", modelPath)
	}

	if err := onnxenv.Init(); err != nil {
		t.Skipf("ONNX Runtime unavailable: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c, err := NewClassifier(modelPath, labelsPath, threshold, "test", logger)
	if err != nil {
		t.Fatalf("NewClassifier: %v", err)
	}
	defer c.Destroy()

	for _, tc := range goldenCases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join("testdata", tc.file)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat fixture: %v", err)
			}

			res, err := c.Classify(path, "root-test", tc.file, info.ModTime().Unix(), info.Size())
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}

			tags := make(map[string]float32, len(res.Tags))
			for _, ts := range res.Tags {
				tags[ts.Label] = ts.Score
			}

			if _, ok := tags[tc.want]; !ok {
				t.Errorf("missing expected tag %q; got %v", tc.want, res.Tags)
			}
			for _, bad := range tc.reject {
				if score, ok := tags[bad]; ok {
					t.Errorf("unexpected tag %q (score %.3f); got %v", bad, score, res.Tags)
				}
			}
		})
	}
}
