package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// Redeploy pulls git (ff-only) and runs cogs redeploy for one app.
func Redeploy(ctx context.Context, wsRoot, appName string) (detail string, err error) {
	if err := gitPullFFOnly(ctx, wsRoot); err != nil {
		return "", err
	}
	cogsBin, err := os.Executable()
	if err != nil {
		return "", err
	}
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return "", err
	}
	if _, err := workspace.ResolveApp(wsRoot, ws, appName); err != nil {
		return "", fmt.Errorf("unknown app %q", appName)
	}
	cmd := exec.CommandContext(ctx, cogsBin, "redeploy", appName, "-y")
	cmd.Dir = wsRoot
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	detail = strings.TrimSpace(string(out))
	if err != nil {
		return detail, fmt.Errorf("redeploy: %w", err)
	}
	remote, branch, commit := gitInfo(wsRoot)
	_ = cockpit.RecordDeploy(wsRoot, cockpit.DeployEvent{
		App: appName, Action: "redeploy", Detail: detail,
		GitRemote: remote, GitBranch: branch, GitCommit: commit,
	})
	return detail, nil
}

func gitInfo(wsRoot string) (remote, branch, commit string) {
	cmd := exec.Command("git", "-C", wsRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if b, err := cmd.Output(); err == nil {
		branch = strings.TrimSpace(string(b))
	}
	cmd = exec.Command("git", "-C", wsRoot, "rev-parse", "--short", "HEAD")
	if b, err := cmd.Output(); err == nil {
		commit = strings.TrimSpace(string(b))
	}
	cmd = exec.Command("git", "-C", wsRoot, "remote", "get-url", "origin")
	if b, err := cmd.Output(); err == nil {
		remote = strings.TrimSpace(string(b))
	}
	return remote, branch, commit
}

func gitPullFFOnly(ctx context.Context, root string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "pull", "--ff-only")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
