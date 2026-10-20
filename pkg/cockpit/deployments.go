package cockpit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// DeployEvent is one launch/redeploy record.
type DeployEvent struct {
	At       time.Time `json:"at"`
	App      string    `json:"app"`
	Action   string    `json:"action"` // launch, redeploy, quick
	Host     string    `json:"host,omitempty"`
	Tunnel   string    `json:"tunnel,omitempty"`
	Pages    string    `json:"pages,omitempty"`
	GitRemote string   `json:"git_remote,omitempty"`
	GitBranch string   `json:"git_branch,omitempty"`
	GitCommit string   `json:"git_commit,omitempty"`
	By       string    `json:"by,omitempty"`
	Detail   string    `json:"detail,omitempty"`
}

// AppMeta is latest known hosting info for an app.
type AppMeta struct {
	App           string    `json:"app"`
	PublicURL     string    `json:"public_url,omitempty"`
	APIURL        string    `json:"api_url,omitempty"`
	PagesProject  string    `json:"pages_project,omitempty"`
	TunnelName    string    `json:"tunnel_name,omitempty"`
	LastDeployAt  time.Time `json:"last_deploy_at"`
	DeployCount   int       `json:"deploy_count"`
	GitRemote     string    `json:"git_remote,omitempty"`
	GitBranch     string    `json:"git_branch,omitempty"`
	GitCommit     string    `json:"git_commit,omitempty"`
}

type deployFile struct {
	Meta   map[string]AppMeta `json:"meta"`
	Events []DeployEvent      `json:"events"`
}

func deployPath(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", "deployments.json")
}

// RecordDeploy appends an event and updates meta (workspace-level).
func RecordDeploy(wsRoot string, ev DeployEvent) error {
	if err := os.MkdirAll(filepath.Join(wsRoot, ".cogs"), 0o750); err != nil {
		return err
	}
	p := deployPath(wsRoot)
	var df deployFile
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &df)
	}
	if df.Meta == nil {
		df.Meta = map[string]AppMeta{}
	}
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	df.Events = append(df.Events, ev)
	if len(df.Events) > 500 {
		df.Events = df.Events[len(df.Events)-500:]
	}
	m := df.Meta[ev.App]
	m.App = ev.App
	m.LastDeployAt = ev.At
	m.DeployCount++
	m.GitRemote = ev.GitRemote
	m.GitBranch = ev.GitBranch
	m.GitCommit = ev.GitCommit
	if ev.Host != "" {
		m.PublicURL = "https://" + ev.Host
	}
	m.TunnelName = ev.Tunnel
	m.PagesProject = ev.Pages
	df.Meta[ev.App] = m
	b, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(p, b, 0o644)
}

// LoadDeployments reads workspace deploy history.
func LoadDeployments(wsRoot string) (deployFile, error) {
	var df deployFile
	b, err := os.ReadFile(deployPath(wsRoot))
	if err != nil {
		if os.IsNotExist(err) {
			df.Meta = map[string]AppMeta{}
			return df, nil
		}
		return df, err
	}
	if err := json.Unmarshal(b, &df); err != nil {
		return df, err
	}
	if df.Meta == nil {
		df.Meta = map[string]AppMeta{}
	}
	return df, nil
}
