// Package hostenv applies the CoGS service user HOME and GOWORK for host CLI tools.
package hostenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/gowork"
)

// ServiceHome is HOME for gh/wrangler when CoGS runs as a dedicated user (e.g. /home/cogs).
func ServiceHome() string {
	if h := strings.TrimSpace(os.Getenv("COGS_HOME")); h != "" {
		return h
	}
	return strings.TrimSpace(os.Getenv("HOME"))
}

// CLIEnv returns os.Environ() with service HOME and GOWORK for a workspace.
func CLIEnv(wsRoot string) []string {
	env := os.Environ()
	home := ServiceHome()
	if home != "" {
		env = setEnv(env, "HOME", home)
	}
	// GOWORK is applied per command via gowork.BuildEnv — not globally here.
	return env
}

func setEnv(env []string, key, val string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return append(out, prefix+val)
}

// CogsCommandContext runs the host cogs CLI (PATH or go run from workspace root).
func CogsCommandContext(ctx context.Context, wsRoot string, args ...string) *exec.Cmd {
	base := append(CLIEnv(wsRoot), "COGS_WORKSPACE_ROOT="+wsRoot)
	env, _ := gowork.BuildEnv(wsRoot, wsRoot, base)
	if bin, err := exec.LookPath("cogs"); err == nil {
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = wsRoot
		cmd.Env = env
		return cmd
	}
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "./cmd/cogs"}, args...)...)
	cmd.Dir = wsRoot
	cmd.Env = env
	return cmd
}

// CloneDir returns the standard path for an app's git clone under the workspace.
func CloneDir(wsRoot, repoFull string) string {
	repoFull = strings.TrimSpace(repoFull)
	return filepath.Join(wsRoot, ".cogs", "clones", strings.ReplaceAll(repoFull, "/", "-"))
}
