package installjob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/appadd"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/appinstall"
	"github.com/irgipaulius/cogs/pkg/gitconfig"
	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/gomod"
	"github.com/irgipaulius/cogs/pkg/gowork"
	"github.com/irgipaulius/cogs/pkg/hostenv"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

const outputTailMax = 12000

type Runner struct {
	WSRoot string
	Store  *Store
	onSave func(*Job)
}

func (r *Runner) Run(ctx context.Context, job *Job) {
	logFile, err := os.OpenFile(r.Store.logPath(job.ID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		r.failJob(job, StageClone, fmt.Errorf("open log: %w", err))
		return
	}
	defer logFile.Close()
	logger := io.MultiWriter(logFile)

	runErr := r.runStages(ctx, job, logger)
	if runErr != nil {
		r.rollback(job)
		r.failJob(job, job.CurrentStage, runErr)
		return
	}
	r.completeJob(job)
}

func (r *Runner) runStages(ctx context.Context, job *Job, log io.Writer) error {
	req := job.Request
	if req.ScaffoldOnly {
		return r.runScaffold(ctx, job, log)
	}
	if strings.TrimSpace(req.FinishApp) != "" {
		return r.runFinishInstall(ctx, job, log)
	}

	if err := r.runStage(ctx, job, StageClone, log, func() error {
		src, err := appinstall.MaterializeRepo(ctx, r.WSRoot, req.RepoFull)
		if err != nil {
			return err
		}
		job.GitRoot = src
		fmt.Fprintf(log, "clone root: %s\n", src)
		return gitconfig.EnsureSafeDirectory(ctx, r.WSRoot, src)
	}); err != nil {
		return err
	}

	if err := r.runStage(ctx, job, StageDiscover, log, func() error {
		cfgPath, err := appinstall.FindAppConfig(job.GitRoot)
		if err != nil {
			return err
		}
		cfgDir := filepath.Dir(cfgPath)
		app, err := appcfg.Load(cfgDir)
		if err != nil {
			return fmt.Errorf("read cogs.app.toml: %w", err)
		}
		name := strings.TrimSpace(req.AppName)
		if name == "" {
			name = app.Name
		}
		if name == "" {
			name = filepath.Base(cfgDir)
		}
		job.AppName = name
		job.AppDir = cfgDir
		fmt.Fprintf(log, "app %s at %s\n", name, cfgDir)
		return nil
	}); err != nil {
		return err
	}

	if err := r.runStage(ctx, job, StagePrepare, log, func() error {
		pageSub := strings.TrimSpace(req.PageSubdomain)
		if req.PageApex {
			pageSub = ""
		}
		if err := appadd.PatchAppMeta(job.AppDir, req.RepoFull, req.DeployMode, req.Zone, pageSub, req.PageApex); err != nil {
			return err
		}
		if _, err := gomod.FixLocalReplace(job.AppDir, r.WSRoot); err != nil {
			return err
		}
		if err := gowork.EnsureOverlayHealthy(r.WSRoot); err != nil {
			return err
		}
		return gitconfig.EnsureSafeDirectory(ctx, r.WSRoot, job.AppDir)
	}); err != nil {
		return err
	}

	if err := r.runStage(ctx, job, StageRegister, log, func() error {
		gitRootRel, _ := filepath.Rel(r.WSRoot, job.GitRoot)
		if strings.HasPrefix(gitRootRel, "..") {
			gitRootRel = ""
		}
		mode := strings.TrimSpace(req.DeployMode)
		if mode == "" {
			if app, err := appcfg.Load(job.AppDir); err == nil {
				mode = app.DeployMode
			}
		}
		if mode == "" {
			mode = "push"
		}
		detail, err := appadd.RegisterFromDir(r.WSRoot, job.AppName, job.AppDir, req.RepoFull, mode, filepath.ToSlash(gitRootRel))
		if err != nil {
			return err
		}
		job.SideEffects.Registered = true
		if rel, e := filepath.Rel(r.WSRoot, job.AppDir); e == nil {
			job.SideEffects.AppPath = filepath.ToSlash(rel)
		}
		r.persist(job)
		fmt.Fprintln(log, detail)
		return nil
	}); err != nil {
		return err
	}

	if err := r.runStage(ctx, job, StageBuild, log, func() error {
		if _, err := os.Stat(filepath.Join(job.AppDir, "go.mod")); err != nil {
			fmt.Fprintln(log, "no go.mod — skip backend build")
			return nil
		}
		rel, err := filepath.Rel(r.WSRoot, job.AppDir)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("app dir outside workspace")
		}
		pkg := "./" + filepath.ToSlash(rel) + "/..."
		cmd := exec.CommandContext(ctx, "go", "build", "-o", os.DevNull, pkg)
		cmd.Dir = r.WSRoot
		env, err := gowork.BuildEnv(r.WSRoot, job.AppDir, hostenv.CLIEnv(r.WSRoot))
		if err != nil {
			return err
		}
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		r.setStageOutput(job, StageBuild, string(out))
		if err != nil {
			return fmt.Errorf("go build: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := r.runStage(ctx, job, StageLaunch, log, func() error {
		cmd := hostenv.CogsCommandContext(ctx, r.WSRoot, "launch", job.AppDir, "-y", "--background-serve")
		out, err := cmd.CombinedOutput()
		r.setStageOutput(job, StageLaunch, string(out))
		fmt.Fprint(log, string(out))
		if err != nil {
			msg := string(out)
			if strings.Contains(msg, "DNS API token is required") {
				return fmt.Errorf("Cloudflare DNS API token missing — set COGS_CF_API_TOKEN on the host or store it under .cogs/cloudflare/ (see launch output in install log)")
			}
			return fmt.Errorf("cogs launch: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := r.runStage(ctx, job, StageHealth, log, func() error {
		url := healthURL(job.AppDir, job.AppName)
		if url == "" {
			fmt.Fprintln(log, "no health URL — assuming launch started process")
			return nil
		}
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if probeHTTPHealth(url) {
				fmt.Fprintf(log, "health OK %s\n", url)
				return nil
			}
			time.Sleep(2 * time.Second)
		}
		return fmt.Errorf("health check timed out for %s", url)
	}); err != nil {
		return err
	}

	go func() {
		c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		ghcli.StarRepoBestEffort(c, req.RepoFull)
	}()
	return nil
}

func (r *Runner) runFinishInstall(ctx context.Context, job *Job, log io.Writer) error {
	name := strings.TrimSpace(job.Request.FinishApp)
	_, ws, err := workspace.FindRoot(r.WSRoot)
	if err != nil {
		return err
	}
	appDir, err := workspace.ResolveApp(r.WSRoot, ws, name)
	if err != nil {
		return err
	}
	job.AppName = name
	job.AppDir = appDir
	job.GitRoot = workspace.GitRootForApp(r.WSRoot, name, "")
	req := job.Request

	if err := r.runStage(ctx, job, StagePrepare, log, func() error {
		if _, err := gomod.FixLocalReplace(job.AppDir, r.WSRoot); err != nil {
			return err
		}
		return gowork.EnsureOverlayHealthy(r.WSRoot)
	}); err != nil {
		return err
	}
	// reuse launch/health from main pipeline — inline call remaining stages
	for _, stage := range []struct {
		id StageID
		fn func() error
	}{
		{StageBuild, func() error {
			if _, err := os.Stat(filepath.Join(job.AppDir, "go.mod")); err != nil {
				return nil
			}
			rel, err := filepath.Rel(r.WSRoot, job.AppDir)
			if err != nil {
				return err
			}
			pkg := "./" + filepath.ToSlash(rel) + "/..."
			cmd := exec.CommandContext(ctx, "go", "build", "-o", os.DevNull, pkg)
			cmd.Dir = r.WSRoot
			env, err := gowork.BuildEnv(r.WSRoot, job.AppDir, hostenv.CLIEnv(r.WSRoot))
			if err != nil {
				return err
			}
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			r.setStageOutput(job, StageBuild, string(out))
			if err != nil {
				return fmt.Errorf("go build: %w", err)
			}
			return nil
		}},
		{StageLaunch, func() error {
			cmd := hostenv.CogsCommandContext(ctx, r.WSRoot, "launch", job.AppDir, "-y", "--background-serve")
			out, err := cmd.CombinedOutput()
			r.setStageOutput(job, StageLaunch, string(out))
			fmt.Fprint(log, string(out))
			if err != nil {
				msg := string(out)
				if strings.Contains(msg, "DNS API token is required") {
					return fmt.Errorf("Cloudflare DNS API token missing — set COGS_CF_API_TOKEN on the host")
				}
				return fmt.Errorf("cogs launch: %w", err)
			}
			return nil
		}},
		{StageHealth, func() error {
			url := healthURL(job.AppDir, job.AppName)
			if url == "" {
				return nil
			}
			deadline := time.Now().Add(90 * time.Second)
			for time.Now().Before(deadline) {
				if probeHTTPHealth(url) {
					return nil
				}
				time.Sleep(2 * time.Second)
			}
			return fmt.Errorf("health check timed out for %s", url)
		}},
	} {
		if err := r.runStage(ctx, job, stage.id, log, stage.fn); err != nil {
			return err
		}
	}
	_ = req
	return nil
}

func (r *Runner) runScaffold(ctx context.Context, job *Job, log io.Writer) error {
	return r.runStage(ctx, job, StageRegister, log, func() error {
		detail, err := appadd.Add(appadd.Options{
			WSRoot:        r.WSRoot,
			Name:          reqName(job),
			GitRepo:       job.Request.RepoFull,
			DeployMode:    job.Request.DeployMode,
			PageSubdomain: job.Request.PageSubdomain,
		})
		if err != nil {
			return err
		}
		job.AppName = reqName(job)
		fmt.Fprintln(log, detail)
		return nil
	})
}

func reqName(job *Job) string {
	if n := strings.TrimSpace(job.Request.AppName); n != "" {
		return n
	}
	return strings.TrimSpace(job.AppName)
}

func healthURL(appDir, appName string) string {
	m, err := launch.Load(appDir)
	if err != nil || m.Port <= 0 {
		return ""
	}
	host := m.BindHost
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d/health", host, m.Port)
}

func probeHTTPHealth(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (r *Runner) runStage(ctx context.Context, job *Job, id StageID, log io.Writer, fn func() error) error {
	job.CurrentStage = id
	idx := job.stageIndex(id)
	if idx >= 0 {
		job.Stages[idx].Status = StatusRunning
		job.Stages[idx].StartedAt = nowUTC()
		job.Stages[idx].Error = ""
	}
	job.UpdatedAt = nowUTC()
	r.persist(job)

	fmt.Fprintf(log, "\n=== %s ===\n", id.Label())
	err := fn()
	end := nowUTC()
	if idx >= 0 {
		job.Stages[idx].EndedAt = end
		if err != nil {
			job.Stages[idx].Status = StatusFailed
			job.Stages[idx].Error = err.Error()
		} else {
			job.Stages[idx].Status = StatusSucceeded
		}
	}
	job.UpdatedAt = end
	r.persist(job)
	return err
}

func (r *Runner) setStageOutput(job *Job, id StageID, out string) {
	idx := job.stageIndex(id)
	if idx < 0 {
		return
	}
	if len(out) > outputTailMax {
		out = out[len(out)-outputTailMax:]
	}
	job.Stages[idx].OutputTail = out
}

func (r *Runner) rollback(job *Job) {
	if !job.SideEffects.Registered || job.AppName == "" {
		return
	}
	_ = workspace.RemoveInstalledApp(r.WSRoot, job.AppName)
	if job.SideEffects.AppPath != "" {
		_ = gowork.RemoveModule(r.WSRoot, job.SideEffects.AppPath)
	}
}

func (r *Runner) failJob(job *Job, stage StageID, err error) {
	job.Status = StatusFailed
	job.Error = err.Error()
	job.CurrentStage = stage
	job.UpdatedAt = nowUTC()
	if idx := job.stageIndex(stage); idx >= 0 && job.Stages[idx].Status == StatusRunning {
		job.Stages[idx].Status = StatusFailed
		job.Stages[idx].Error = err.Error()
		job.Stages[idx].EndedAt = nowUTC()
	}
	r.persist(job)
}

func (r *Runner) completeJob(job *Job) {
	job.Status = StatusSucceeded
	job.CurrentStage = StageDone
	job.UpdatedAt = nowUTC()
	if idx := job.stageIndex(StageDone); idx >= 0 {
		job.Stages[idx].Status = StatusSucceeded
		job.Stages[idx].StartedAt = nowUTC()
		job.Stages[idx].EndedAt = nowUTC()
	}
	r.persist(job)
}

func (r *Runner) persist(job *Job) {
	_ = r.Store.Save(job)
	if r.onSave != nil {
		r.onSave(job)
	}
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
