package reconcile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/release"
)

func TestExpandKeepSetIncludesCurrentPointer(t *testing.T) {
	root := t.TempDir()
	app := "goldilocks"
	commit := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	dir := release.AppRoot(root, app)
	if err := os.MkdirAll(filepath.Join(dir, commit), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = release.Activate(root, app, commit)
	keep := expandKeepSet(root, app, false, nil)
	if !keepReleaseSHA(keep, commit) {
		t.Fatalf("keep set missing current commit %q: %v", commit, keep)
	}
}
