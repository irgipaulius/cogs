package appinstall

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/apppath"
)

// skipWalkDir names excluded from config discovery (build artifacts, not source apps).
func skipWalkDir(name string) bool {
	switch name {
	case ".cogs", ".git", "node_modules", "vendor", "dist", "internal", "ui":
		return true
	}
	return false
}

// WorkspaceAppDir returns apps/<id> when present in the monorepo checkout.
func WorkspaceAppDir(wsRoot, appID string) (string, bool) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return "", false
	}
	dir := filepath.Join(wsRoot, "apps", appID)
	if _, err := os.Stat(filepath.Join(dir, appcfg.Filename)); err == nil {
		return dir, true
	}
	return "", false
}

// ResolveAppDir locates the canonical app directory for reconcile (never under .cogs/worktrees).
func ResolveAppDir(ctx context.Context, wsRoot, appID, repo string) (appDir, gitRootRel string, err error) {
	wsRoot = strings.TrimSpace(wsRoot)
	appID = strings.TrimSpace(appID)
	if dir, ok := WorkspaceAppDir(wsRoot, appID); ok {
		return dir, "", nil
	}
	if ws, err := loadWorkspaceAppPath(wsRoot, appID); err == nil && ws != "" {
		dir := filepath.Join(wsRoot, filepath.FromSlash(ws))
		if _, err := os.Stat(filepath.Join(dir, appcfg.Filename)); err == nil && !apppath.IsEphemeral(dir) {
			return dir, "", nil
		}
	}
	src, err := MaterializeRepo(ctx, wsRoot, repo)
	if err != nil {
		return "", "", err
	}
	cfgPath, err := FindAppConfigForApp(src, appID)
	if err != nil {
		return "", "", err
	}
	appDir = filepath.Dir(cfgPath)
	if apppath.IsEphemeral(appDir) {
		return "", "", fmt.Errorf("refusing ephemeral app dir %s", appDir)
	}
	gitRel, _ := filepath.Rel(wsRoot, src)
	if strings.HasPrefix(gitRel, "..") {
		gitRel = ""
	}
	return appDir, filepath.ToSlash(gitRel), nil
}

func loadWorkspaceAppPath(wsRoot, appID string) (string, error) {
	b, err := os.ReadFile(filepath.Join(wsRoot, "cogs.workspace.json"))
	if err != nil {
		return "", err
	}
	type app struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	type doc struct {
		Apps []app `json:"apps"`
	}
	var d doc
	if err := json.Unmarshal(b, &d); err != nil {
		return "", err
	}
	for _, a := range d.Apps {
		if a.Name == appID && strings.TrimSpace(a.Path) != "" {
			return filepath.ToSlash(a.Path), nil
		}
	}
	return "", fmt.Errorf("not in workspace")
}
