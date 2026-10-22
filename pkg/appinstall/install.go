package appinstall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appadd"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/ghcli"
)

// Options for installing an app from a GitHub repository.
type Options struct {
	WSRoot        string
	RepoFull      string // owner/name
	AppName       string // workspace app id; inferred if empty
	DeployMode    string
	PageSubdomain string
	Zone          string
}

// Install registers the app and returns the workspace app name and directory path.
func Install(ctx context.Context, opts Options) (name, appDir, detail string, err error) {
	opts.RepoFull = strings.TrimSpace(opts.RepoFull)
	if opts.RepoFull == "" {
		return "", "", "", fmt.Errorf("repo required")
	}
	st, err := ghcli.Status(ctx)
	if err != nil {
		return "", "", "", err
	}
	if !st.LoggedIn {
		return "", "", "", fmt.Errorf("GitHub not logged in on this host — run: gh auth login")
	}

	srcRoot, err := materializeRepo(ctx, opts.WSRoot, opts.RepoFull)
	if err != nil {
		return "", "", "", err
	}
	cfgPath, err := findAppConfig(srcRoot)
	if err != nil {
		return "", "", "", err
	}
	cfgDir := filepath.Dir(cfgPath)
	app, err := appcfg.Load(cfgDir)
	if err != nil {
		return "", "", "", fmt.Errorf("read cogs.app.toml: %w", err)
	}
	name = strings.TrimSpace(opts.AppName)
	if name == "" {
		name = app.Name
	}
	if name == "" {
		name = filepath.Base(cfgDir)
	}
	mode := strings.TrimSpace(opts.DeployMode)
	if mode == "" {
		mode = app.DeployMode
	}
	if mode == "" {
		mode = "push"
	}

	detail, err = appadd.RegisterFromDir(opts.WSRoot, name, cfgDir, opts.RepoFull, mode)
	if err != nil {
		return "", "", "", err
	}
	appDir = cfgDir
	return name, appDir, detail, nil
}

func materializeRepo(ctx context.Context, wsRoot, fullName string) (string, error) {
	if remote, err := workspaceOrigin(wsRoot); err == nil && repoMatches(remote, fullName) {
		return wsRoot, nil
	}
	dest := filepath.Join(wsRoot, ".cogs", "clones", strings.ReplaceAll(fullName, "/", "-"))
	if isGitCheckout(dest) {
		cmd := exec.CommandContext(ctx, "git", "-C", dest, "pull", "--ff-only")
		_, _ = cmd.CombinedOutput()
		return dest, nil
	}
	_ = os.RemoveAll(dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	bin, err := exec.LookPath("gh")
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin, "repo", "clone", fullName, dest)
	cmd.Dir = wsRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh repo clone: %w: %s", err, string(out))
	}
	return dest, nil
}

func isGitCheckout(dir string) bool {
	return exec.Command("git", "-C", dir, "rev-parse", "HEAD").Run() == nil
}

func findAppConfig(root string) (string, error) {
	if p := filepath.Join(root, appcfg.Filename); fileExists(p) {
		return p, nil
	}
	var found string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) != appcfg.Filename {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"apps"+string(filepath.Separator)) {
			if found == "" {
				found = path
			}
		}
		return nil
	})
	if found != "" {
		return found, nil
	}
	return "", fmt.Errorf("no %s found in %s", appcfg.Filename, root)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func workspaceOrigin(wsRoot string) (string, error) {
	cmd := exec.Command("git", "-C", wsRoot, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func repoMatches(remote, fullName string) bool {
	remote = strings.ToLower(remote)
	fullName = strings.ToLower(fullName)
	return strings.Contains(remote, fullName) || strings.Contains(remote, strings.ReplaceAll(fullName, "/", "-"))
}
