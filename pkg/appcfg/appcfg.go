// Package appcfg loads cogs.app.toml, the per-app source of truth.
package appcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const Filename = "cogs.app.toml"

// App is the committed description of one CoGS app.
type App struct {
	Name           string
	Kind           string
	Zone           string
	PagesDir       string
	PageSubdomain  string
	APISubdomain   string
	BindHost       string
	Port           int
	D1Name         string
	R2Bucket       string
	TunnelName     string
	PagesProject   string
	MaxUploadBytes int64
	Attachments    bool
	BlobKeyPrefix  string
	DeployMode     string // push | pull
	DeployBranch   string
	GitRepo        string
	WorkerName     string // Cloudflare Worker for static UI (split apps)
	RootDirectory  string // monorepo path in git repo for builds
}

const (
	DeployPush = "push"
	DeployPull = "pull"
)

// Load reads cogs.app.toml. Missing file returns os.ErrNotExist.
func Load(dir string) (App, error) {
	b, err := os.ReadFile(filepath.Join(dir, Filename))
	if err != nil {
		return App{}, err
	}
	return Parse(string(b))
}

// Parse reads a flat TOML document (key = value, # comments).
func Parse(s string) (App, error) {
	var a App
	for i, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			return App{}, fmt.Errorf("cogs.app.toml:%d: tables are not used; keep keys flat", i+1)
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return App{}, fmt.Errorf("cogs.app.toml:%d: want key = value", i+1)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if err := apply(&a, key, val); err != nil {
			return App{}, fmt.Errorf("cogs.app.toml:%d: %w", i+1, err)
		}
	}
	if a.Name == "" {
		return App{}, fmt.Errorf("cogs.app.toml: name is required")
	}
	if a.DeployMode == "" {
		a.DeployMode = DeployPush
	}
	if a.DeployBranch == "" {
		a.DeployBranch = "main"
	}
	if a.DeployMode == DeployPull && a.Kind == "combined" {
		return App{}, fmt.Errorf("cogs.app.toml: deploy_mode pull is not allowed for kind combined")
	}
	return a, nil
}

// Validate checks deploy mode rules.
func (a App) Validate() error {
	if a.DeployMode != DeployPush && a.DeployMode != DeployPull {
		return fmt.Errorf("deploy_mode must be push or pull")
	}
	if a.DeployMode == DeployPull && a.Kind == "combined" {
		return fmt.Errorf("pull mode requires a split frontend Worker")
	}
	return nil
}

func apply(a *App, key, raw string) error {
	switch key {
	case "name":
		a.Name = unquote(raw)
	case "kind":
		a.Kind = unquote(raw)
	case "zone":
		a.Zone = unquote(raw)
	case "pages_dir":
		a.PagesDir = unquote(raw)
	case "page_subdomain":
		a.PageSubdomain = unquote(raw)
	case "api_subdomain":
		a.APISubdomain = unquote(raw)
	case "bind_host":
		a.BindHost = unquote(raw)
	case "port":
		n, err := strconv.Atoi(unquote(raw))
		if err != nil {
			return err
		}
		a.Port = n
	case "d1_name":
		a.D1Name = unquote(raw)
	case "r2_bucket":
		a.R2Bucket = unquote(raw)
	case "tunnel_name":
		a.TunnelName = unquote(raw)
	case "pages_project":
		a.PagesProject = unquote(raw)
	case "max_upload_bytes":
		n, err := strconv.ParseInt(unquote(raw), 10, 64)
		if err != nil {
			return err
		}
		a.MaxUploadBytes = n
	case "attachments":
		a.Attachments = raw == "true"
	case "blob_key_prefix":
		a.BlobKeyPrefix = unquote(raw)
	case "deploy_mode":
		a.DeployMode = unquote(raw)
	case "deploy_branch":
		a.DeployBranch = unquote(raw)
	case "git_repo":
		a.GitRepo = unquote(raw)
	case "worker_name":
		a.WorkerName = unquote(raw)
	case "root_directory":
		a.RootDirectory = unquote(raw)
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// KindManifest is the launch kind value. "split" and "pages" mean a separate UI host.
func (a App) KindManifest() string {
	if a.Kind == "split" || a.Kind == "pages" {
		return ""
	}
	return a.Kind
}

// PageHost is subdomain.zone, or the zone apex when page_subdomain is empty.
func (a App) PageHost() string {
	zone := strings.TrimSpace(a.Zone)
	if zone == "" {
		return ""
	}
	sub := strings.TrimSpace(a.PageSubdomain)
	if sub == "" {
		return zone
	}
	return sub + "." + zone
}

// APIHost is the public API hostname. Combined apps use the page host.
func (a App) APIHost() string {
	if a.Kind == "combined" {
		return a.PageHost()
	}
	if a.Zone == "" || a.APISubdomain == "" {
		return ""
	}
	return a.APISubdomain + "." + a.Zone
}
