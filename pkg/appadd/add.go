package appadd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// Options for scaffolding a new workspace app.
type Options struct {
	WSRoot        string
	Name          string
	GitRepo       string
	DeployMode    string
	PageSubdomain string
	APISubdomain  string
	Zone          string
}

// Add creates apps/<name>/cogs.app.toml and registers the app in cogs.workspace.json + go.work.
func Add(opts Options) (string, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	if opts.Name == "" {
		return "", fmt.Errorf("name required")
	}
	if opts.DeployMode == "" {
		opts.DeployMode = appcfg.DeployPush
	}
	_, ws, err := workspace.FindRoot(opts.WSRoot)
	if err != nil {
		return "", err
	}
	for _, a := range ws.Apps {
		if a.Name == opts.Name {
			return "", fmt.Errorf("app %q already exists", opts.Name)
		}
	}
	appDir := filepath.Join(opts.WSRoot, "apps", opts.Name)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	if opts.PageSubdomain == "" {
		opts.PageSubdomain = opts.Name
	}
	if opts.APISubdomain == "" {
		opts.APISubdomain = opts.Name + "-api"
	}
	if opts.Zone == "" {
		opts.Zone = "example.com"
	}
	worker := "cogs-" + opts.Name
	toml := fmt.Sprintf(`name = %q
kind = "split"
zone = %q
pages_dir = "public"
page_subdomain = %q
api_subdomain = %q
bind_host = "127.0.0.1"
port = 0
tunnel_name = %q
pages_project = %q
worker_name = %q
deploy_mode = %q
deploy_branch = "main"
git_repo = %q
`, opts.Name, opts.Zone, opts.PageSubdomain, opts.APISubdomain,
		"cogs-"+opts.Name, worker, worker, opts.DeployMode, opts.GitRepo)
	if err := os.WriteFile(filepath.Join(appDir, appcfg.Filename), []byte(toml), 0o644); err != nil {
		return "", err
	}
	ws.Apps = append(ws.Apps, workspace.App{
		Name: opts.Name, Path: "apps/" + opts.Name,
		Description: "Added from admin dashboard",
	})
	b, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(filepath.Join(opts.WSRoot, workspace.FileName), b, 0o644); err != nil {
		return "", err
	}
	rel := "./apps/" + opts.Name
	cmd := exec.Command("go", "work", "use", rel)
	cmd.Dir = opts.WSRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go work use: %w: %s", err, string(out))
	}
	return fmt.Sprintf("created %s", appDir), nil
}

// AppDir resolves apps/<name>.
func AppDir(wsRoot, name string) (string, error) {
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return "", err
	}
	return workspace.ResolveApp(wsRoot, ws, name)
}
