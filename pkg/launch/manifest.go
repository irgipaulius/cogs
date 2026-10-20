// Package launch describes a project for `cogs launch`.
package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DefaultFilename = "cogs.launch.json"

// Manifest configures Cloudflare resources and local ports for guided deploy.
type Manifest struct {
	Name           string `json:"name"`
	PagesDir       string `json:"pages_dir"`
	PageSubdomain  string `json:"page_subdomain"`  // todo -> todo.example.com
	APISubdomain   string `json:"api_subdomain"`   // todo-api -> todo-api.example.com
	BindHost       string `json:"bind_host,omitempty"`
	Port           int    `json:"port"`
	D1Name         string `json:"d1_name"`
	R2Bucket       string `json:"r2_bucket"`
	TunnelName     string `json:"tunnel_name"`
	PagesProject   string `json:"pages_project"`
	MaxUploadBytes int64  `json:"max_upload_bytes"`
}

// Load reads cogs.launch.json from dir.
func Load(dir string) (Manifest, error) {
	path := filepath.Join(dir, DefaultFilename)
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	m.applyDefaults()
	return m, nil
}

func (m *Manifest) applyDefaults() {
	if m.PagesDir == "" {
		m.PagesDir = "public"
	}
	if m.PageSubdomain == "" {
		m.PageSubdomain = "todo"
	}
	if m.APISubdomain == "" {
		m.APISubdomain = m.PageSubdomain + "-api"
	}
	if m.BindHost == "" {
		m.BindHost = "127.0.0.1"
	}
	// Port 0 = workspace auto-assign (see pkg/workspace AssignPorts).
	if m.D1Name == "" {
		m.D1Name = m.Name + "-edge"
	}
	if m.R2Bucket == "" {
		m.R2Bucket = m.Name + "-images"
	}
	if m.TunnelName == "" {
		m.TunnelName = "cogs-" + m.Name
	}
	if m.PagesProject == "" {
		m.PagesProject = m.Name
	}
	if m.MaxUploadBytes == 0 {
		m.MaxUploadBytes = 12 << 20 // 12 MiB
	}
}
