package reconcile

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cfclient"
	"github.com/irgipaulius/cogs/pkg/cfprovision"
	"github.com/irgipaulius/cogs/pkg/release"
	"github.com/irgipaulius/cogs/pkg/state"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func gcApp(ctx context.Context, st *state.Store, appID string) error {
	wsRoot := st.WSRoot()
	app, err := st.GetApp(appID)
	if err != nil {
		return err
	}
	c := &cfclient.Client{WSRoot: wsRoot}
	if err := cfprovision.Teardown(ctx, c, st, appID); err != nil {
		return err
	}
	if app.AppPath != "" {
		dir := filepath.Join(wsRoot, filepath.FromSlash(app.AppPath))
		_ = os.RemoveAll(filepath.Join(dir, ".env"))
		_ = os.RemoveAll(filepath.Join(dir, ".cogs"))
	}
	if app.GitRoot != "" {
		_ = os.RemoveAll(filepath.Join(wsRoot, filepath.FromSlash(app.GitRoot)))
	}
	_ = os.RemoveAll(filepath.Join(wsRoot, ".cogs", "releases", appID))
	_ = os.RemoveAll(filepath.Join(wsRoot, ".cogs", "worktrees", appID))
	_ = os.RemoveAll(filepath.Join(wsRoot, ".cogs", "build", appID))
	_ = os.RemoveAll(filepath.Join(wsRoot, ".cogs", "logs", "reconcile", appID))
	if err := workspace.UnregisterApp(wsRoot, appID); err != nil {
		return err
	}
	_ = st.ClearHostnames(appID)
	return st.DeleteApp(appID)
}

func pruneReleases(st *state.Store, appID string) error {
	wsRoot := st.WSRoot()
	combined := releaseCombined(st, appID)
	keepSHAs, err := st.KeepReleaseSHAs(appID, 3)
	if err != nil {
		return err
	}
	keep := expandKeepSet(wsRoot, appID, combined, keepSHAs)
	rels, err := st.ListReleaseRows(appID)
	if err != nil {
		return err
	}
	for _, r := range rels {
		if keepReleaseSHA(keep, r.CommitSHA) {
			continue
		}
		resolved := release.ResolveCommit(wsRoot, appID, r.CommitSHA)
		if resolved != "" {
			_ = os.RemoveAll(filepath.Join(release.AppRoot(wsRoot, appID), resolved))
		}
		_ = st.DeleteReleaseRow(appID, r.ID)
	}
	appRoot := release.AppRoot(wsRoot, appID)
	entries, _ := os.ReadDir(appRoot)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "current" || name == "previous" {
			continue
		}
		if !keepReleaseSHA(keep, name) {
			_ = os.RemoveAll(filepath.Join(appRoot, name))
		}
	}
	wtRoot := filepath.Join(wsRoot, ".cogs", "worktrees", appID)
	wtEntries, _ := os.ReadDir(wtRoot)
	for _, e := range wtEntries {
		if e.IsDir() && !keepReleaseSHA(keep, strings.TrimSpace(e.Name())) {
			_ = os.RemoveAll(filepath.Join(wtRoot, e.Name()))
		}
	}
	buildRoot := filepath.Join(wsRoot, ".cogs", "build", appID)
	_ = os.RemoveAll(buildRoot)
	return nil
}

func releaseCombined(st *state.Store, appID string) bool {
	app, err := st.GetApp(appID)
	if err != nil || app.AppPath == "" {
		return appID == "admin"
	}
	dir := filepath.Join(st.WSRoot(), filepath.FromSlash(app.AppPath))
	if cfg, err := appcfg.Load(dir); err == nil {
		return cfg.Kind == "combined"
	}
	return appID == "admin"
}

func expandKeepSet(wsRoot, appID string, combined bool, shas []string) map[string]bool {
	keep := map[string]bool{}
	for _, sha := range shas {
		sha = strings.TrimSpace(sha)
		if sha == "" {
			continue
		}
		if full := release.ResolveCommit(wsRoot, appID, sha); full != "" {
			keep[full] = true
		} else if release.BinaryExists(wsRoot, appID, sha, combined) {
			keep[sha] = true
		}
	}
	return keep
}

func keepReleaseSHA(keep map[string]bool, sha string) bool {
	return keep[strings.TrimSpace(sha)]
}
