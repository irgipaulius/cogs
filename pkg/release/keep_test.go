package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCurrentSHANeverEmptyWhenPointerSet(t *testing.T) {
	root := t.TempDir()
	app := "goldilocks"
	commit := "abc123def456789012345678901234567890abcd"
	dir := AppRoot(root, app)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, currentFile), []byte(commit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ReadCurrentSHA(root, app)
	if got != commit {
		t.Fatalf("ReadCurrentSHA=%q want %q", got, commit)
	}
}
