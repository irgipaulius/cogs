package reconcile

import (
	"context"
	"os"
	"path/filepath"
	"strings"

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
	rels, err := st.ListReleaseRows(appID)
	if err != nil {
		return err
	}
	keepSHAs, err := st.KeepReleaseSHAs(appID, 3)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, sha := range keepSHAs {
		keep[sha] = true
	}
	for _, r := range rels {
		if keep[r.CommitSHA] {
			continue
		}
		_ = os.RemoveAll(filepath.Join(release.AppRoot(wsRoot, appID), r.CommitSHA))
		_ = st.DeleteReleaseRow(appID, r.ID)
	}
	wtRoot := filepath.Join(wsRoot, ".cogs", "worktrees", appID)
	entries, _ := os.ReadDir(wtRoot)
	for _, e := range entries {
		if e.IsDir() && !keep[strings.TrimSpace(e.Name())] {
			_ = os.RemoveAll(filepath.Join(wtRoot, e.Name()))
		}
	}
	buildRoot := filepath.Join(wsRoot, ".cogs", "build", appID)
	_ = os.RemoveAll(buildRoot)
	return nil
}
