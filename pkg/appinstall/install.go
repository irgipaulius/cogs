package appinstall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/appadd"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/gitpublic"
	"github.com/irgipaulius/cogs/pkg/hostenv"
)

func hostenvClone(wsRoot, fullName string) string {
	return hostenv.CloneDir(wsRoot, fullName)
}

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
	opts.RepoFull = gitpublic.NormalizeFullName(opts.RepoFull)
	if opts.RepoFull == "" {
		return "", "", "", fmt.Errorf("repo required (owner/name)")
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

	gitRootRel, _ := filepath.Rel(opts.WSRoot, srcRoot)
	if strings.HasPrefix(gitRootRel, "..") {
		gitRootRel = ""
	}
	if err := appadd.PatchAppMeta(cfgDir, opts.RepoFull, mode, opts.Zone, opts.PageSubdomain); err != nil {
		return "", "", "", err
	}
	detail, err = appadd.RegisterFromDir(opts.WSRoot, name, cfgDir, opts.RepoFull, mode, filepath.ToSlash(gitRootRel))
	if err != nil {
		return "", "", "", err
	}
	appDir = cfgDir
	repoToStar := opts.RepoFull
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		ghcli.StarRepoBestEffort(c, repoToStar)
	}()
	return name, appDir, detail, nil
}

func materializeRepo(ctx context.Context, wsRoot, fullName string) (string, error) {
	if remote, err := workspaceOrigin(wsRoot); err == nil && repoMatches(remote, fullName) {
		return wsRoot, nil
	}
	dest := hostenvClone(wsRoot, fullName)
	if isGitCheckout(dest) {
		if st, _ := ghcli.Status(ctx); st.LoggedIn {
			cmd := exec.CommandContext(ctx, "git", "-C", dest, "pull", "--ff-only")
			cmd.Env = hostenv.CLIEnv(wsRoot)
			_, _ = cmd.CombinedOutput()
		}
		return dest, nil
	}
	_ = os.RemoveAll(dest)

	st, _ := ghcli.Status(ctx)
	if st.LoggedIn {
		if err := cloneWithGH(ctx, wsRoot, fullName, dest); err == nil {
			return dest, nil
		}
	}
	if gitpublic.AccessibleWithoutAuth(ctx, fullName) {
		if err := gitpublic.CloneHTTPS(ctx, fullName, dest); err != nil {
			return "", err
		}
		return dest, nil
	}
	if _, err := gitpublic.LookupUnauthenticated(ctx, fullName); err != nil {
		return "", err
	}
	return "", fmt.Errorf("cannot clone %s — run gh auth login on the host for private repositories", fullName)
}

func cloneWithGH(ctx context.Context, wsRoot, fullName, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	bin, err := exec.LookPath("gh")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "repo", "clone", fullName, dest)
	cmd.Dir = wsRoot
	cmd.Env = hostenv.CLIEnv(wsRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh repo clone: %w: %s", err, string(out))
	}
	return nil
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
