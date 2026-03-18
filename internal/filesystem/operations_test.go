package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDelete(t *testing.T) {
	t.Run("delete file", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "test.txt")
		writeFile(t, f, "hello")

		if err := Delete(f, false); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Error("file should not exist after delete")
		}
	})

	t.Run("delete empty dir", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "empty")
		os.Mkdir(sub, 0o755)

		if err := Delete(sub, false); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("non-empty dir without recursive fails", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "sub", "file.txt"), "data")

		err := Delete(filepath.Join(dir, "sub"), false)
		if err == nil {
			t.Error("should fail for non-empty dir without recursive")
		}
	})

	t.Run("non-empty dir with recursive", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")
		writeFile(t, filepath.Join(sub, "file.txt"), "data")

		if err := Delete(sub, true); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(sub); !os.IsNotExist(err) {
			t.Error("directory should not exist after recursive delete")
		}
	})
}

func TestFileChecksum(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	f3 := filepath.Join(dir, "c.txt")
	writeFile(t, f1, "hello")
	writeFile(t, f2, "hello")
	writeFile(t, f3, "world")

	sum1, _ := FileChecksum(f1)
	sum2, _ := FileChecksum(f2)
	sum3, _ := FileChecksum(f3)

	if sum1 != sum2 {
		t.Error("identical files should have same checksum")
	}
	if sum1 == sum3 {
		t.Error("different files should have different checksums")
	}
}

func TestCopyFile(t *testing.T) {
	t.Run("simple copy", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeFile(t, src, "content")

		actual, err := CopyFile(src, dst)
		if err != nil {
			t.Fatal(err)
		}
		if actual != dst {
			t.Errorf("expected dst=%s, got %s", dst, actual)
		}

		data, _ := os.ReadFile(dst)
		if string(data) != "content" {
			t.Error("copy content mismatch")
		}
	})

	t.Run("dedup identical", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeFile(t, src, "same")
		writeFile(t, dst, "same")

		actual, err := CopyFile(src, dst)
		if err != nil {
			t.Fatal(err)
		}
		if actual != dst {
			t.Error("identical copy should return original dst path")
		}
	})

	t.Run("auto-rename on conflict", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "file.jpg")
		dst := filepath.Join(dir, "file.jpg") // same name but we need different dirs
		// Actually use different dirs
		srcDir := filepath.Join(dir, "a")
		dstDir := filepath.Join(dir, "b")
		src = filepath.Join(srcDir, "file.jpg")
		dst = filepath.Join(dstDir, "file.jpg")
		writeFile(t, src, "new version")
		writeFile(t, dst, "old version")

		actual, err := CopyFile(src, dst)
		if err != nil {
			t.Fatal(err)
		}
		expected := filepath.Join(dstDir, "file (2).jpg")
		if actual != expected {
			t.Errorf("expected %s, got %s", expected, actual)
		}

		data, _ := os.ReadFile(actual)
		if string(data) != "new version" {
			t.Error("renamed copy should have source content")
		}
	})
}

func TestCopyDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	writeFile(t, filepath.Join(src, "a.txt"), "aaa")
	writeFile(t, filepath.Join(src, "sub", "b.txt"), "bbb")

	dst := filepath.Join(dir, "dst")
	if err := CopyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil || string(data) != "aaa" {
		t.Error("a.txt content mismatch")
	}
	data, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil || string(data) != "bbb" {
		t.Error("sub/b.txt content mismatch")
	}
}

func TestMoveFile(t *testing.T) {
	t.Run("simple move", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeFile(t, src, "moved")

		actual, err := MoveFile(src, dst)
		if err != nil {
			t.Fatal(err)
		}
		if actual != dst {
			t.Errorf("expected %s, got %s", dst, actual)
		}

		if _, err := os.Stat(src); !os.IsNotExist(err) {
			t.Error("source should not exist after move")
		}
		data, _ := os.ReadFile(dst)
		if string(data) != "moved" {
			t.Error("move content mismatch")
		}
	})

	t.Run("move identical dedup", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "a", "file.txt")
		dst := filepath.Join(dir, "b", "file.txt")
		writeFile(t, src, "same")
		writeFile(t, dst, "same")

		_, err := MoveFile(src, dst)
		if err != nil {
			t.Fatal(err)
		}
		// Source should be deleted
		if _, err := os.Stat(src); !os.IsNotExist(err) {
			t.Error("source should be deleted on identical move")
		}
	})

	t.Run("move dir", func(t *testing.T) {
		dir := t.TempDir()
		src := filepath.Join(dir, "srcdir")
		writeFile(t, filepath.Join(src, "f.txt"), "data")

		dst := filepath.Join(dir, "dstdir")
		actual, err := MoveFile(src, dst)
		if err != nil {
			t.Fatal(err)
		}
		if actual != dst {
			t.Errorf("expected %s, got %s", dst, actual)
		}
		if _, err := os.Stat(src); !os.IsNotExist(err) {
			t.Error("source dir should not exist after move")
		}
		data, _ := os.ReadFile(filepath.Join(dst, "f.txt"))
		if string(data) != "data" {
			t.Error("moved dir content mismatch")
		}
	})
}
