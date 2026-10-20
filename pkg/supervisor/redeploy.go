package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	_ = filepath.Join(wsRoot, "apps")
	return detail, nil
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
