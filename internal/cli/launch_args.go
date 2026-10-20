package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

type launchFlags struct {
	dir         string
	skipPages   bool
	skipTunnel  bool
	noTunnelBG  bool
	yes         bool
	skipUIBuild bool
	pickZone    bool
	quick       bool
}

func parseLaunchFlags(args []string) (launchFlags, error) {
	fs := flag.NewFlagSet("launch", flag.ExitOnError)
	f := launchFlags{}
	fs.StringVar(&f.dir, "dir", ".", "project directory with cogs.launch.json")
	fs.BoolVar(&f.skipPages, "skip-pages", false, "skip Pages deploy")
	fs.BoolVar(&f.skipTunnel, "skip-tunnel", false, "skip tunnel DNS setup")
	fs.BoolVar(&f.noTunnelBG, "no-tunnel-bg", false, "do not start tunnel in background")
	fs.BoolVar(&f.yes, "yes", false, "skip confirmation prompt")
	fs.BoolVar(&f.yes, "y", false, "skip confirmation prompt")
	fs.BoolVar(&f.skipUIBuild, "skip-ui-build", false, "skip npm build when frontend/ exists")
	fs.BoolVar(&f.pickZone, "pick-zone", false, "always show zone picker (default: reuse .env hostnames)")
	fs.BoolVar(&f.quick, "quick", false, "redeploy only: UI build, Pages, migrate, serve (skip Cloudflare provisioning)")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if rest := fs.Args(); len(rest) > 0 {
		f.dir = rest[0]
	}
	return f, nil
}

func resolveLaunchProjectDir(dir string) (string, error) {
	if dir == "" || dir == "." {
		dir = "."
	}
	candidates := []string{dir}
	if wd, err := os.Getwd(); err == nil && !filepath.IsAbs(dir) {
		candidates = append(candidates, filepath.Join(wd, dir))
	}
	seen := map[string]struct{}{}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		if hasLaunchManifest(abs) {
			return abs, nil
		}
		if p, ok := resolveViaWorkspace(abs, dir); ok {
			return p, nil
		}
		if p, ok := resolveViaWorkspace(abs, c); ok {
			return p, nil
		}
	}
	return "", fmt.Errorf("no %s for %q (use app name from cogs.workspace.json, apps/<name>, or a path with %s)", launch.DefaultFilename, dir, launch.DefaultFilename)
}

func hasLaunchManifest(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, launch.DefaultFilename))
	return err == nil
}

func resolveViaWorkspace(fromDir, appRef string) (string, bool) {
	rootDir, ws, err := workspace.FindRoot(fromDir)
	if err != nil {
		return "", false
	}
	resolved, err := workspace.ResolveApp(rootDir, ws, appRef)
	if err != nil {
		return "", false
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", false
	}
	if !hasLaunchManifest(abs) {
		return "", false
	}
	return abs, true
}

func isLaunchProjectPath(arg string) bool {
	if arg == "" || arg[0] == '-' {
		return false
	}
	if _, err := resolveLaunchProjectDir(arg); err == nil {
		return true
	}
	return false
}
