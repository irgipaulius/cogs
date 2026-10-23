package reconcile

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/appadd"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/appinstall"
	"github.com/irgipaulius/cogs/pkg/cfprovision"
	"github.com/irgipaulius/cogs/pkg/deploy"
	"github.com/irgipaulius/cogs/pkg/gitconfig"
	"github.com/irgipaulius/cogs/pkg/gomod"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/release"
	"github.com/irgipaulius/cogs/pkg/state"
)

// Options controls one reconcile run.
type Options struct {
	Commit       string
	ForceRebuild bool
	StagesOnly   []string // empty = all
}

// Run executes the reconcile pipeline for one app.
func Run(ctx context.Context, st *state.Store, appID string, opts Options) error {
	app, err := st.GetApp(appID)
	if err != nil {
		return err
	}
	unlock, err := lockApp(st.WSRoot(), appID)
	if err != nil {
		return err
	}
	defer unlock()

	logPath := filepath.Join(st.WSRoot(), ".cogs", "logs", "reconcile", appID, fmt.Sprintf("%d.log", time.Now().Unix()))
	_ = os.MkdirAll(filepath.Dir(logPath), 0o750)
	logFile, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	logger := io.MultiWriter(logFile)
	if logFile != nil {
		defer logFile.Close()
	}

	runID, err := st.BeginRun(appID, logPath)
	if err != nil {
		return err
	}
	_ = st.SetConverged(appID, false, "")

	runErr := runPipeline(ctx, st, app, runID, opts, logger)
	if runErr != nil {
		_ = st.FinishRun(runID, "failed", runErr.Error())
		_ = st.SetConverged(appID, false, runErr.Error())
		return runErr
	}
	_ = st.FinishRun(runID, "succeeded", "")
	_ = st.SetConverged(appID, true, "")
	return nil
}

func runPipeline(ctx context.Context, st *state.Store, app state.AppView, runID string, opts Options, log io.Writer) error {
	wsRoot := st.WSRoot()
	if !app.Present {
		return stage(st, runID, "gc", log, func() error { return gcApp(ctx, st, app.ID) })
	}

	var appDir string
	var tunnelID string
	cfCfg := cfprovision.NewConfig(wsRoot, log)

	if err := stage(st, runID, "source", log, func() error {
		src, err := appinstall.MaterializeRepo(ctx, wsRoot, app.Repo)
		if err != nil {
			return err
		}
		cfgPath, err := appinstall.FindAppConfig(src)
		if err != nil {
			return err
		}
		appDir = filepath.Dir(cfgPath)
		_ = gitconfig.EnsureSafeDirectory(ctx, wsRoot, src)
		gitRel, _ := filepath.Rel(wsRoot, src)
		if strings.HasPrefix(gitRel, "..") {
			gitRel = ""
		}
		appRel, _ := filepath.Rel(wsRoot, appDir)
		if strings.HasPrefix(appRel, "..") {
			return fmt.Errorf("app dir outside workspace")
		}
		d := app.AppDesired
		d.Present = true
		d.AppPath = filepath.ToSlash(appRel)
		d.GitRoot = filepath.ToSlash(gitRel)
		if err := appadd.PatchAppMeta(appDir, app.Repo, app.DeployMode, app.Zone, app.PageSubdomain, app.PageSubdomain == "" && app.Zone != ""); err != nil {
			return err
		}
		_, err = appadd.RegisterFromDir(wsRoot, app.ID, appDir, app.Repo, app.DeployMode, d.GitRoot)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return err
		}
		return st.SetDesired(d)
	}); err != nil {
		return err
	}

	if err := stage(st, runID, "validate", log, func() error {
		if appDir == "" {
			var e error
			appDir, e = resolveAppDir(wsRoot, app)
			if e != nil {
				return e
			}
		}
		if err := st.CheckHostnames(app.ID, app.PageSubdomain, app.APISubdomain, app.Zone); err != nil {
			return err
		}
		return gomod.CheckLocalReplace(appDir)
	}); err != nil {
		return err
	}

	if err := stage(st, runID, "build_workspace", log, func() error {
		if _, err := gomod.FixLocalReplace(appDir, wsRoot); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	if err := stage(st, runID, "secrets", log, func() error {
		m, err := launch.Load(appDir)
		if err != nil {
			return err
		}
		pageHost, apiHost := cfprovision.Hosts(app, m)
		return cfprovision.EnsureEnv(appDir, m, pageHost, apiHost)
	}); err != nil {
		return err
	}

	if err := stage(st, runID, "cf_resources", log, func() error {
		return cfprovision.EnsureD1R2(ctx, cfCfg, st, app, appDir)
	}); err != nil {
		return err
	}

	if err := stage(st, runID, "tunnel", log, func() error {
		var err error
		tunnelID, err = cfprovision.EnsureTunnel(ctx, cfCfg, st, app, appDir)
		return err
	}); err != nil {
		return err
	}

	if err := stage(st, runID, "worker", log, func() error {
		return cfprovision.EnsureWorker(ctx, cfCfg, st, app, appDir)
	}); err != nil {
		return err
	}

	if err := stage(st, runID, "dns", log, func() error {
		return cfprovision.EnsureDNS(ctx, cfCfg, st, app, appDir, tunnelID)
	}); err != nil {
		return err
	}

	buildErr := stage(st, runID, "build_release", log, func() error {
		detail, err := deploy.DeployBackend(ctx, wsRoot, deploy.Options{
			AppID:  app.ID,
			Commit: opts.Commit,
			Mode:   app.DeployMode,
			Action: "reconcile",
		})
		if err != nil {
			return err
		}
		fmt.Fprintln(log, detail)
		return nil
	})
	if buildErr != nil {
		cur, _, _ := st.CurrentRelease(app.ID)
		if cur != "" {
			short := cur
			if len(short) > 7 {
				short = short[:7]
			}
			msg := fmt.Sprintf("build failed (still serving %s): %v", short, buildErr)
			fmt.Fprintln(log, msg)
			_ = st.SetConverged(app.ID, false, msg)
			return fmt.Errorf("%s", msg)
		}
		return buildErr
	}

	if err := stage(st, runID, "migrate", log, func() error {
		m, _ := launch.Load(appDir)
		bin, ok := release.CurrentBinary(wsRoot, app.ID, m.IsCombined())
		if !ok {
			return fmt.Errorf("migrate: no release binary")
		}
		cmd := exec.CommandContext(ctx, bin, "migrate", "-env", ".env")
		cmd.Dir = appDir
		out, err := cmd.CombinedOutput()
		fmt.Fprint(log, string(out))
		if err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	_ = stage(st, runID, "activate", log, func() error { return nil })
	return stage(st, runID, "gc", log, func() error { return pruneReleases(st, app.ID) })
}

func stage(st *state.Store, runID, name string, log io.Writer, fn func() error) error {
	_ = st.UpdateRunStage(runID, name, "running")
	fmt.Fprintf(log, "\n=== %s ===\n", name)
	err := fn()
	status := "succeeded"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}
	_ = st.RecordStage(runID, name, status, "", errMsg)
	if err != nil {
		return err
	}
	return nil
}

func resolveAppDir(wsRoot string, app state.AppView) (string, error) {
	if app.AppPath != "" {
		return filepath.Join(wsRoot, filepath.FromSlash(app.AppPath)), nil
	}
	if _, err := appcfg.Load(filepath.Join(wsRoot, "apps", app.ID)); err == nil {
		return filepath.Join(wsRoot, "apps", app.ID), nil
	}
	return "", fmt.Errorf("app path unknown for %s", app.ID)
}
