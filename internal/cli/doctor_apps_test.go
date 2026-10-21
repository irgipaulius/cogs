package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/workspace"
)

func TestConfigDeclares(t *testing.T) {
	body := `window.COGS_CONFIG = { pageOrigin: "https://todo.hyperreader.eu", apiBase: "https://todo-api.hyperreader.eu" };`
	if !configDeclares(body, "todo.hyperreader.eu", "todo-api.hyperreader.eu") {
		t.Fatal("expected match")
	}
	if configDeclares(body, "todo.hyperreader.eu", "todo.hyperreader.eu/api") {
		t.Fatal("relative api must not match")
	}
}

func TestDoctorGoWork(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "apps", "goldilocks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/goldilocks\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := doctorOne(dir, workspace.App{Name: "goldilocks", Path: "apps/goldilocks"}, "use (\n\t./apps/todo\n)\n")
	for _, f := range findings {
		if !f.OK && stringIndex(f.Problem, "does not include") >= 0 {
			if f.Fix != "go work use ./apps/goldilocks" {
				t.Fatal(f.Fix)
			}
			return
		}
	}
	t.Fatalf("missing go.work finding: %+v", findings)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
