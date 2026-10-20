package cli

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/launch"
)

func recordLaunchDeploy(wsRoot, projectDir, action string, m launch.Manifest, pageHost, apiHost string) {
	ev := cockpit.DeployEvent{
		App:    filepath.Base(projectDir),
		Action: action,
		Tunnel: m.TunnelName,
		Pages:  m.PagesProject,
	}
	if m.Name != "" {
		ev.App = m.Name
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
