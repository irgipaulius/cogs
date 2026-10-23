package gitrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldSkipDeployCommitGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "t@test")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "f")
	run("commit", "-m", "c1")
	old := gitHead(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "f")
	run("commit", "-m", "c2")
	cur := gitHead(t, dir)

	ctx := context.Background()
	if !ShouldSkipDeployCommit(ctx, dir, old, cur) {
		t.Fatal("expected old skipped when current is newer")
	}
	if !ShouldSkipDeployCommit(ctx, dir, cur, cur) {
		t.Fatal("same commit should skip")
	}
	if ShouldSkipDeployCommit(ctx, dir, cur, old) {
		t.Fatal("should not skip newer commit")
	}
}

func gitHead(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
