package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

type deployLocalOpts struct {
	apps        []string
	quick       bool
	yes         bool
	ping        bool
	parallel    bool
	skipPages   bool
	skipTunnel  bool
	skipUIBuild bool
	noTunnelBG  bool
	serveBG     bool
}

func cmdDeployWorkspace(args []string) error {
	opts, err := parseDeployLocalFlags(args)
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	rootDir, ws, err := workspace.FindRoot(wd)
	if err != nil {
		return fmt.Errorf("run from CoGS workspace root (need %s): %w", workspace.FileName, err)
	}
	targets, err := filterWorkspaceApps(ws, opts.apps)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no apps to deploy")
	}

	var plan []string
	for _, a := range targets {
		dir := filepath.Join(rootDir, a.Path)
		m, _ := launch.Load(dir)
		resolveListen(dir, &m)
		plan = append(plan, fmt.Sprintf("%s → %s (%s)", a.Name, listenAddr(m.BindHost, m.Port), a.Path))
	}
	ux.Plan("Deploy plan", plan)
	if globals.DryRun {
		ux.OK("dry-run: no changes made")
		return nil
	}

	if !opts.quick {
		if err := ensureWranglerLogin(); err != nil {
			return err
		}
	}

	runOne := func(a workspace.App) error {
		dir := filepath.Join(rootDir, a.Path)
		lf := launchFlags{
			dir:         dir,
			skipPages:   opts.skipPages,
			skipTunnel:  opts.skipTunnel,
			noTunnelBG:  opts.noTunnelBG,
			yes:         opts.yes,
			skipUIBuild: opts.skipUIBuild,
			quick:       opts.quick,
		}
		lf.serveMode = serveBackground
		if !opts.serveBG {
			lf.serveMode = serveSkip
		}
		if err := runLaunch(lf); err != nil {
			return fmt.Errorf("%s: %w", a.Name, err)
		}
		if opts.ping {
			return pingApp(dir, a.Name)
		}
		return nil
	}

	start := time.Now()
	if opts.parallel && len(targets) > 1 {
		var wg sync.WaitGroup
		errCh := make(chan error, len(targets))
		for _, a := range targets {
			a := a
			wg.Add(1)
			go func() {
				defer wg.Done()
				errCh <- runOne(a)
			}()
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				return err
			}
		}
	} else {
		for _, a := range targets {
			if err := runOne(a); err != nil {
				return err
			}
		}
	}
	ux.OK(fmt.Sprintf("deploy finished %d app(s) in %s", len(targets), time.Since(start).Round(time.Millisecond)))
	return nil
}

func parseDeployLocalFlags(args []string) (deployLocalOpts, error) {
	o := deployLocalOpts{
		quick:    true,
		yes:      true,
		serveBG:  true,
		parallel: true,
	}
	var apps []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--full":
			o.quick = false
		case "--quick":
			o.quick = true
		case "--yes", "-y":
			o.yes = true
		case "--ping":
			o.ping = true
		case "--parallel":
			o.parallel = true
		case "--serial":
			o.parallel = false
		case "--skip-pages":
			o.skipPages = true
		case "--skip-tunnel":
			o.skipTunnel = true
		case "--skip-ui-build":
			o.skipUIBuild = true
		case "--no-tunnel-bg":
			o.noTunnelBG = true
		case "--foreground-serve":
			o.serveBG = false
		case "--app":
			if i+1 >= len(args) {
				return o, fmt.Errorf("--app requires a name")
			}
			i++
			apps = append(apps, args[i])
		default:
			if strings.HasPrefix(a, "--app=") {
				apps = append(apps, strings.TrimPrefix(a, "--app="))
			} else if !strings.HasPrefix(a, "-") {
				apps = append(apps, a)
			}
		}
	}
	if len(apps) > 0 {
		o.apps = apps
	}
	return o, nil
}

func filterWorkspaceApps(ws workspace.Root, names []string) ([]workspace.App, error) {
	if len(names) == 0 {
		return ws.Apps, nil
	}
	var out []workspace.App
	for _, n := range names {
		found := false
		for _, a := range ws.Apps {
			if a.Name == n || a.Path == n || strings.HasSuffix(a.Path, "/"+n) {
				out = append(out, a)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown app %q (cogs apps list)", n)
		}
	}
	return out, nil
}

func pingApp(projectDir, name string) error {
	m, err := launch.Load(projectDir)
	if err != nil {
		return err
	}
	resolveListen(projectDir, &m)
	url := "http://" + listenAddr(m.BindHost, m.Port) + "/health"
	client := &http.Client{Timeout: 4 * time.Second}
	var last error
	for i := 0; i < 8; i++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ux.AppOK(name, "ping /health ok")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		last = err
		time.Sleep(400 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("health not ok")
	}
	return fmt.Errorf("ping %s: %w", url, last)
}
