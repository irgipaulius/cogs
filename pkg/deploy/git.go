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

// ResolveCommit returns full SHA for ref or commit in the app's git repository.
func ResolveCommit(ctx context.Context, gitRoot, ref string, token string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}
	if err := gitFetchOrigin(ctx, gitRoot, token); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", gitRoot, "rev-parse", ref)
	cmd.Env = gitEnv(token)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitFetchOrigin(ctx context.Context, gitRoot, token string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", gitRoot, "fetch", "origin", "--prune")
	cmd.Env = gitEnv(token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureWorktree(ctx context.Context, wsRoot, gitRoot, appID, commit, token string) (string, error) {
	dir := worktreeDir(wsRoot, appID, commit)
	if st, err := os.Stat(filepath.Join(dir, ".git")); err == nil && !st.IsDir() {
		return dir, nil
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return dir, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return "", err
	}
	if !hasCommit(ctx, gitRoot, commit) {
		if err := gitFetchOrigin(ctx, gitRoot, token); err != nil {
			return "", err
		}
	}
	if !hasCommit(ctx, gitRoot, commit) {
		return "", fmt.Errorf("commit %s not found after fetch in %s", shortSHA(commit), gitRoot)
	}
	_ = exec.CommandContext(ctx, "git", "-C", gitRoot, "worktree", "remove", "--force", dir).Run()
	cmd := exec.CommandContext(ctx, "git", "-C", gitRoot, "worktree", "add", "--detach", dir, commit)
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

func hasCommit(ctx context.Context, gitRoot, commit string) bool {
	return exec.CommandContext(ctx, "git", "-C", gitRoot, "cat-file", "-e", commit+"^{commit}").Run() == nil
}

func gitEnv(token string) []string {
	if token != "" {
		return append(os.Environ(), githubauth.ConfigureGitEnv(token)...)
	}
	return os.Environ()
}
