package cli

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func recordLaunchDeploy(wsRoot, projectDir, action string, m launch.Manifest, pageHost, apiHost string) {
	appID := filepath.Base(projectDir)
	if wsRoot != "" {
		if _, ws, err := workspace.FindRoot(projectDir); err == nil {
			appID = workspace.CanonicalID(wsRoot, ws, projectDir)
		}
	}
	ev := cockpit.DeployEvent{
		App: appID, Action: action, Status: "succeeded",
		Tunnel: m.TunnelName, Pages: m.PagesProject, Mode: m.DeployMode,
	}
	if ev.Mode == "" {
		if app, err := appcfg.Load(projectDir); err == nil {
			ev.Mode = app.DeployMode
		}
	}
	if m.IsCombined() {
		ev.Host = pageHost
	} else {
		ev.Host = pageHost
		ev.Detail = "api=" + apiHost
	}
	ev.GitRemote, ev.GitBranch, ev.GitCommit = gitInfo(wsRoot)
	_ = cockpit.RecordDeploy(wsRoot, ev)
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
