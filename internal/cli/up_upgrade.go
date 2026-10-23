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
	head := strings.TrimSpace(workspaceGitHead(root))
	if head == "" {
		head = strings.TrimSpace(version.Commit)
	}
	if len(head) < 7 {
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
		if cur == head {
			continue
		}
		if _, _, ok := release.BinaryForCommit(root, a.Name, head, appCombined(root, a.Path)); ok && cur == head {
			continue
		}
		rec.Enqueue(a.Name, reconcile.Options{Commit: head, ForceRebuild: true})
	}
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
