package buildenv

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureModCacheOutsideWorkspace(t *testing.T) {
	ws := t.TempDir()
	inside := filepath.Join(ws, "go", "pkg", "mod")
	env := EnsureModCacheOutsideWorkspace(ws, []string{"GOMODCACHE=" + inside})
	got := envVal(env, "GOMODCACHE")
	if got == inside || pathWithin(got, ws) {
		t.Fatalf("GOMODCACHE still inside workspace: %q", got)
	}
	if !strings.Contains(got, "cogs-gomodcache") && !strings.Contains(got, string(filepath.Separator)+"go"+string(filepath.Separator)) {
		t.Fatalf("unexpected cache path %q", got)
	}
}
