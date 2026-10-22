package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/githubauth"
)

func worktreeDir(wsRoot, appID, commit string) string {
	short := commit
	if len(short) > 12 {
		short = short[:12]
	}
	return filepath.Join(wsRoot, ".cogs", "worktrees", appID, short)
}

// ResolveCommit returns full SHA for ref or commit in the workspace repo.
func ResolveCommit(ctx context.Context, wsRoot, ref string, token string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}
	if err := gitFetchOrigin(ctx, wsRoot, token); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", wsRoot, "rev-parse", ref)
	cmd.Env = gitEnv(token)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitFetchOrigin(ctx context.Context, wsRoot, token string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", wsRoot, "fetch", "origin", "--prune")
	cmd.Env = gitEnv(token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureWorktree(ctx context.Context, wsRoot, appID, commit, token string) (string, error) {
	dir := worktreeDir(wsRoot, appID, commit)
	if st, err := os.Stat(filepath.Join(dir, ".git")); err == nil && !st.IsDir() {
		return dir, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return "", err
	}
	// Queue events carry a full SHA that may be newer than our last fetch.
	if !hasCommit(ctx, wsRoot, commit) {
		if err := gitFetchOrigin(ctx, wsRoot, token); err != nil {
			return "", err
		}
	}
	_ = exec.CommandContext(ctx, "git", "-C", wsRoot, "worktree", "remove", "--force", dir).Run()
	cmd := exec.CommandContext(ctx, "git", "-C", wsRoot, "worktree", "add", "--detach", dir, commit)
	cmd.Env = gitEnv(token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return dir, nil
}

func branchRef(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	return "origin/" + branch
}

func hasCommit(ctx context.Context, wsRoot, commit string) bool {
	return exec.CommandContext(ctx, "git", "-C", wsRoot, "cat-file", "-e", commit+"^{commit}").Run() == nil
}

// gitEnv uses gh/git credential helper when token is empty (host gh auth setup-git).
func gitEnv(token string) []string {
	if token != "" {
		return append(os.Environ(), githubauth.ConfigureGitEnv(token)...)
	}
	return os.Environ()
}
