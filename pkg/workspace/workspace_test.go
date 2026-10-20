package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootAndResolveApp(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "apps", "todo")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"name":"test","apps":[{"name":"todo","path":"apps/todo"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	root, got, err := FindRoot(appDir)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("root=%q want %q", root, dir)
	}
	if len(got.Apps) != 1 || got.Apps[0].Name != "todo" {
		t.Fatalf("apps=%+v", got.Apps)
	}
	p, err := ResolveApp(root, got, "todo")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(p) != filepath.Clean(appDir) {
		t.Fatalf("path=%q want %q", p, appDir)
	}
}
