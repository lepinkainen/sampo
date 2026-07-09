package handlers

import (
	"testing"

	"github.com/lepinkainen/sampo/internal/filesystem"
)

func TestDirListCacheCopiesOnPutAndGet(t *testing.T) {
	cache := newDirListCache()
	entries := []filesystem.FileEntry{{Name: "one", Path: "one"}}

	cache.put("root-0", "/", entries)

	v := true
	entries[0].HasPerson = &v
	entries[0].Name = "mutated-after-put"

	got, ok := cache.get("root-0", "/")
	if !ok {
		t.Fatal("cache miss")
	}
	if got[0].Name != "one" {
		t.Fatalf("cached name = %q, want original", got[0].Name)
	}
	if got[0].HasPerson != nil {
		t.Fatalf("cached HasPerson = %v, want nil", *got[0].HasPerson)
	}

	got[0].Name = "mutated-after-get"
	got[0].Tags = []filesystem.TagScore{{Label: "stale", Score: 1}}

	gotAgain, ok := cache.get("root-0", "/")
	if !ok {
		t.Fatal("cache miss on second get")
	}
	if gotAgain[0].Name != "one" {
		t.Fatalf("cached name after get mutation = %q, want original", gotAgain[0].Name)
	}
	if gotAgain[0].Tags != nil {
		t.Fatalf("cached Tags after get mutation = %+v, want nil", gotAgain[0].Tags)
	}
}
