package wranglerexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateWorkspaceLogTruncatesLargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrangler.log")
	b := make([]byte, workspaceWranglerLogMax+1024)
	for i := range b {
		b[i] = 'x'
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	RotateWorkspaceLog(path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > workspaceWranglerLogMax {
		t.Fatalf("log still %d bytes", info.Size())
	}
}
