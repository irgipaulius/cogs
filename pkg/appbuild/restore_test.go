package appbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreTrackedUIArtifactsChecksOutPublicConfig(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "t@example.com")
	runGit(t, root, "config", "user.name", "t")
	app := filepath.Join(root, "apps", "todo")
	pub := filepath.Join(app, "public")
	_ = os.MkdirAll(pub, 0o755)
	cfg := filepath.Join(pub, "config.js")
	if err := os.WriteFile(cfg, []byte("window.COGS_CONFIG = { apiBase: \"https://prod\" };\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "apps/todo/public/config.js")
	runGit(t, root, "commit", "-m", "baseline")
	if err := os.WriteFile(cfg, []byte("dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RestoreTrackedUIArtifacts(app, "public"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	b, _ := os.ReadFile(cfg)
	if !contains(string(b), "prod") {
		t.Fatalf("config not restored: %q", string(b))
	}
}

func TestRemoveUntrackedUIHashAssets(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "t@example.com")
	runGit(t, root, "config", "user.name", "t")
	app := filepath.Join(root, "apps", "todo")
	assets := filepath.Join(app, "public", "assets")
	_ = os.MkdirAll(assets, 0o755)
	tracked := filepath.Join(assets, "index-tracked.js")
	junk := filepath.Join(assets, "index-deadbeef.js")
	_ = os.WriteFile(tracked, []byte("// t\n"), 0o644)
	_ = os.WriteFile(junk, []byte("// j\n"), 0o644)
	runGit(t, root, "add", "apps/todo/public/assets/index-tracked.js")
	runGit(t, root, "commit", "-m", "assets")
	if err := RemoveUntrackedUIHashAssets(app, "public"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(junk); !os.IsNotExist(err) {
		t.Fatalf("junk file remains: %v", err)
	}
	if _, err := os.Stat(tracked); err != nil {
		t.Fatalf("tracked removed: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || strings.Contains(s, sub))
}
