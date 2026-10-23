package cli

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/reconcile"
	"github.com/irgipaulius/cogs/pkg/release"
	"github.com/irgipaulius/cogs/pkg/state"
	"github.com/irgipaulius/cogs/pkg/version"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func enqueueMonorepoRebuildIfNeeded(root string, st *state.Store, rec *reconcile.Manager, ws workspace.Root) {
	if rec == nil || st == nil {
		return
	}
	target := strings.TrimSpace(workspaceGitHead(root))
	cliCommit := strings.TrimSpace(version.Commit)
	if len(cliCommit) >= 7 {
		if target == "" || !commitsEqual(target, cliCommit) {
			target = cliCommit
		}
	}
	if len(target) < 7 {
		return
	}
	for _, a := range ws.Apps {
		if !isWorkspaceMonorepoApp(root, a.Path) {
			continue
		}
		cur, _ := release.ReadCurrent(root, a.Name)
		if cur != "" {
			if resolved := release.ResolveCommit(root, a.Name, cur); resolved != "" {
				cur = resolved
			}
		}
		if commitsEqual(cur, target) {
			continue
		}
		rec.Enqueue(a.Name, reconcile.Options{Commit: target, ForceRebuild: true})
	}
}

func commitsEqual(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(a, b) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(a), strings.ToLower(b)) ||
		strings.HasPrefix(strings.ToLower(b), strings.ToLower(a))
}

func workspaceGitHead(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isWorkspaceMonorepoApp(root, appPath string) bool {
	dir := filepath.Join(root, filepath.FromSlash(appPath))
	cfg, err := appcfg.Load(dir)
	if err != nil {
		return true
	}
	repo := strings.TrimSpace(cfg.GitRepo)
	if repo == "" {
		return true
	}
	return strings.Contains(strings.ToLower(repo), "cogs")
}

func appCombined(root, appPath string) bool {
	dir := filepath.Join(root, filepath.FromSlash(appPath))
	if cfg, err := appcfg.Load(dir); err == nil {
		return cfg.Kind == "combined"
	}
	return filepath.Base(appPath) == "admin"
}
