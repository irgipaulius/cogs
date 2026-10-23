package appbuild

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasFrontend(t *testing.T) {
	dir := t.TempDir()
	if HasFrontend(dir) {
		t.Fatal("expected false")
	}
	_ = os.MkdirAll(filepath.Join(dir, "frontend"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "frontend", "package.json"), []byte("{}"), 0o644)
	if !HasFrontend(dir) {
		t.Fatal("expected true")
	}
}
