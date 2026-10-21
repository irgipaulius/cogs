package supervisor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/release"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func enrichDeployMetaFromLaunch(wsRoot string, ws workspace.Root, meta map[string]cockpit.AppMeta) {
	zone := strings.TrimSpace(os.Getenv("COGS_PUBLIC_ZONE"))
	for _, a := range ws.Apps {
		dir := filepath.Join(wsRoot, a.Path)
		m, err := launch.Load(dir)
		if err != nil {
			continue
		}
		cur := meta[a.Name]
		if cur.App == "" {
			cur.App = a.Name
		}
		if cur.PagesProject == "" {
			cur.PagesProject = m.PagesProject
		}
		if cur.TunnelName == "" {
			cur.TunnelName = m.TunnelName
		}
		if app, err := appcfg.Load(dir); err == nil {
			if cur.DeployMode == "" {
				cur.DeployMode = app.DeployMode
			}
		}
		if commit, _ := release.ReadCurrent(wsRoot, a.Name); commit != "" {
			if len(commit) > 7 {
				commit = commit[:7]
			}
			cur.GitCommit = commit
		}
		if zone != "" {
			if cur.PublicURL == "" && m.PageSubdomain != "" {
				cur.PublicURL = "https://" + m.PageSubdomain + "." + zone
			}
			if !m.IsCombined() && cur.APIURL == "" && m.APISubdomain != "" {
				cur.APIURL = "https://" + m.APISubdomain + "." + zone
			}
		}
		if cur.GitBranch == "" || cur.GitCommit == "" {
			if g := gitHead(wsRoot); g != nil {
				if cur.GitBranch == "" {
					cur.GitBranch = g.branch
				}
				if cur.GitCommit == "" {
					cur.GitCommit = g.commit
				}
			}
		}
		meta[a.Name] = cur
	}
}

type gitSnapshot struct {
	branch string
	commit string
}

func gitHead(wsRoot string) *gitSnapshot {
	// Best-effort: workspace is bind-mounted in Docker; git may be absent in image.
	head, err := os.ReadFile(filepath.Join(wsRoot, ".git", "HEAD"))
	if err != nil {
		return nil
	}
	line := strings.TrimSpace(string(head))
	if strings.HasPrefix(line, "ref: ") {
		ref := strings.TrimPrefix(line, "ref: ")
		b, err := os.ReadFile(filepath.Join(wsRoot, ".git", ref))
		if err != nil {
			return &gitSnapshot{branch: filepath.Base(ref)}
		}
		c := strings.TrimSpace(string(b))
		if len(c) > 7 {
			c = c[:7]
		}
		return &gitSnapshot{branch: filepath.Base(ref), commit: c}
	}
	if len(line) > 7 {
		line = line[:7]
	}
	if line != "" {
		return &gitSnapshot{commit: line}
	}
	return nil
}
