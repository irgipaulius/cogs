package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileIfChangedSkipsIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if err := WriteFileIfChanged(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info2, _ := os.Stat(path)
	if !info.ModTime().Equal(info2.ModTime()) {
		t.Fatal("mtime changed on identical write")
	}
}
