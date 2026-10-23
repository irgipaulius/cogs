package provision

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/gomod"
	"github.com/irgipaulius/cogs/pkg/release"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// State describes whether an app is safe to treat as production-ready on this host.
type State struct {
	App     string   `json:"app"`
	Ready   bool     `json:"ready"`
	Status  string   `json:"status"` // ready, incomplete, not_provisioned
	Reasons []string `json:"reasons,omitempty"`
}

// Assess inspects on-disk signals for one workspace app.
func Assess(wsRoot, appName, appDir string) State {
	st := State{App: appName, Status: "not_provisioned"}
	if appDir == "" {
		st.Reasons = append(st.Reasons, "app directory unknown")
		return st
	}
	if _, err := os.Stat(filepath.Join(appDir, ".env")); err != nil {
		st.Reasons = append(st.Reasons, "never launched — missing .env (run install or cogs launch)")
	}
	if err := gomod.CheckLocalReplace(appDir); err != nil {
		st.Reasons = append(st.Reasons, err.Error())
	}
	cur, _ := release.ReadCurrent(wsRoot, appName)
	if cur == "" {
		if _, err := os.Stat(filepath.Join(appDir, ".cogs", "run", release.BinaryName(false))); err != nil {
			if cfg, e := appcfg.Load(appDir); e == nil && cfg.Kind == "combined" {
				if _, err2 := os.Stat(filepath.Join(appDir, ".cogs", "run", release.BinaryName(true))); err2 != nil {
					st.Reasons = append(st.Reasons, "no release binary — deploy or launch has not succeeded")
				}
			} else {
				st.Reasons = append(st.Reasons, "no release binary — deploy or launch has not succeeded")
			}
		}
	}
	df, _ := cockpit.LoadDeployments(wsRoot)
	if meta, ok := df.Meta[appName]; ok {
		if meta.LastStatus != "succeeded" {
			if meta.LastStatus == "failed" {
				st.Reasons = append(st.Reasons, "last deploy failed: "+strings.TrimSpace(meta.LastError))
			} else if meta.DeployCount == 0 {
				st.Reasons = append(st.Reasons, "no successful deploy recorded")
			}
		}
	} else if _, err := os.Stat(filepath.Join(appDir, ".env")); err != nil {
		// already noted missing .env
	}
	if len(st.Reasons) == 0 {
		st.Ready = true
		st.Status = "ready"
		return st
	}
	st.Status = "incomplete"
	return st
}

// AssessAll returns provision state for every app in the workspace manifest.
func AssessAll(wsRoot string) map[string]State {
	out := map[string]State{}
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return out
	}
	for _, a := range ws.Apps {
		if a.Name == "admin" {
			continue
		}
		dir, err := workspace.ResolveApp(wsRoot, ws, a.Name)
		if err != nil {
			out[a.Name] = State{App: a.Name, Status: "incomplete", Reasons: []string{err.Error()}}
			continue
		}
		out[a.Name] = Assess(wsRoot, a.Name, dir)
	}
	return out
}

// InstalledByRepo finds a workspace app id for a git remote slug owner/name.
func InstalledByRepo(wsRoot, repoFull string) string {
	repoFull = strings.ToLower(strings.TrimSpace(repoFull))
	if repoFull == "" {
		return ""
	}
	for _, ia := range workspace.ListInstalledApps(wsRoot) {
		if strings.ToLower(strings.TrimSpace(ia.GitRepo)) == repoFull {
			return ia.Name
		}
	}
	return ""
}
