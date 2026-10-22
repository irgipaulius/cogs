package deploy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/githubauth"
	"github.com/irgipaulius/cogs/pkg/procspawn"
	"github.com/irgipaulius/cogs/pkg/release"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// Options controls a backend deploy at a specific commit.
type Options struct {
	AppID   string
	Commit  string // empty = tip of deploy branch
	Mode    string // push | pull | rollback
	Action  string // redeploy, rollback, pull_build, launch
	By      string
	Restart func(appDir, binPath string) error // optional; default spawns via release binary
}

// DeployBackend builds at commit, health-checks, activates release pointer, restarts app.
func DeployBackend(ctx context.Context, wsRoot string, opts Options) (detail string, err error) {
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return "", err
	}
	appDir, err := workspace.ResolveApp(wsRoot, ws, opts.AppID)
	if err != nil {
		return "", err
	}
	appID := workspace.CanonicalID(wsRoot, ws, appDir)
	cfg, err := appcfg.Load(appDir)
	if err != nil {
		return "", fmt.Errorf("app config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	token := githubauth.Token(wsRoot)
	commit := strings.TrimSpace(opts.Commit)
	if commit == "" {
		commit, err = ResolveCommit(ctx, wsRoot, branchRef(cfg.DeployBranch), token)
		if err != nil {
			return "", err
		}
	} else if len(commit) < 40 {
		full, err := ResolveCommit(ctx, wsRoot, commit, token)
		if err == nil {
			commit = full
		}
	}
	mode := opts.Mode
	if mode == "" {
		mode = cfg.DeployMode
	}
	action := opts.Action
	if action == "" {
		action = "redeploy"
	}
	ev := cockpit.DeployEvent{
		App: appID, Action: action, Mode: mode, Status: "running",
		GitCommit: commit, GitBranch: cfg.DeployBranch, GitRemote: cfg.GitRepo,
		By: opts.By,
	}
	_ = cockpit.RecordDeploy(wsRoot, ev)

	wtRoot, err := ensureWorktree(ctx, wsRoot, appID, commit, token)
	if err != nil {
		return failDeploy(wsRoot, ev, err)
	}
	buildDir := appDir
	if rel := strings.TrimSpace(cfg.RootDirectory); rel != "" {
		buildDir = filepath.Join(wtRoot, filepath.FromSlash(rel))
	} else {
		// Monorepo: same relative path under worktree.
		relPath, err := filepath.Rel(wsRoot, appDir)
		if err == nil && relPath != "." {
			buildDir = filepath.Join(wtRoot, relPath)
		} else {
			buildDir = filepath.Join(wtRoot, filepath.Base(appDir))
		}
	}
	outBin := release.BinaryPath(wsRoot, appID, commit, cfg.Kind == "combined")
	if err := os.MkdirAll(filepath.Dir(outBin), 0o755); err != nil {
		return failDeploy(wsRoot, ev, err)
	}
	if err := goBuild(ctx, buildDir, outBin); err != nil {
		return failDeploy(wsRoot, ev, err)
	}
	// Smoke-test the candidate from the live app dir: .env (gitignored, so absent
	// from the worktree) and relative data paths resolve exactly as after restart.
	if err := healthCheck(ctx, appDir, outBin); err != nil {
		return failDeploy(wsRoot, ev, err)
	}
	prev, _ := release.ReadCurrent(wsRoot, appID)
	if err := release.Activate(wsRoot, appID, commit); err != nil {
		return failDeploy(wsRoot, ev, err)
	}
	restart := opts.Restart
	if restart == nil {
		restart = defaultRestart(appDir)
	}
	if err := restart(appDir, outBin); err != nil {
		// Roll back pointer on restart failure.
		if prev != "" {
			_ = release.Activate(wsRoot, appID, prev)
		}
		return failDeploy(wsRoot, ev, err)
	}
	ev.Status = "succeeded"
	ev.Detail = fmt.Sprintf("commit %s → %s", shortSHA(commit), outBin)
	_ = cockpit.RecordDeploy(wsRoot, ev)
	return ev.Detail, nil
}

// RollbackBackend swaps to the previous release commit.
func RollbackBackend(ctx context.Context, wsRoot, appID string, restart func(appDir, binPath string) error) (detail string, err error) {
	prev, err := release.ReadPrevious(wsRoot, appID)
	if err != nil || prev == "" {
		return "", fmt.Errorf("no previous release to roll back to")
	}
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return "", err
	}
	appDir, err := workspace.ResolveApp(wsRoot, ws, appID)
	if err != nil {
		return "", err
	}
	cfg, err := appcfg.Load(appDir)
	if err != nil {
		return "", err
	}
	bin := release.BinaryPath(wsRoot, appID, prev, cfg.Kind == "combined")
	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		return "", fmt.Errorf("previous binary missing at %s", bin)
	}
	if err := release.Activate(wsRoot, appID, prev); err != nil {
		return "", err
	}
	if restart == nil {
		restart = defaultRestart(appDir)
	}
	if err := restart(appDir, bin); err != nil {
		return "", err
	}
	ev := cockpit.DeployEvent{
		App: appID, Action: "rollback", Status: "succeeded", GitCommit: prev,
		Detail: "rolled back to " + shortSHA(prev),
	}
	_ = cockpit.RecordDeploy(wsRoot, ev)
	return ev.Detail, nil
}

func failDeploy(wsRoot string, ev cockpit.DeployEvent, err error) (string, error) {
	ev.Status = "failed"
	ev.Detail = err.Error()
	_ = cockpit.RecordDeploy(wsRoot, ev)
	return ev.Detail, err
}

func goBuild(ctx context.Context, appDir, outBin string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outBin, "./cmd/app")
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func healthCheck(ctx context.Context, appDir, bin string) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "serve", "-env", ".env")
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(), "COGS_LISTEN="+addr)
	logFile, err := os.CreateTemp("", "cogs-health-*.log")
	if err != nil {
		return err
	}
	logPath := logFile.Name()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = procspawn.Detached()
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	_ = logFile.Close()
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	url := "http://" + addr + "/health"
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				_ = os.Remove(logPath)
				return nil
			}
			return fmt.Errorf("health %d: %s (%s)", resp.StatusCode, strings.TrimSpace(string(body)), tailLogFile(logPath))
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("health check timed out for %s (%s)", url, tailLogFile(logPath))
}

func tailLogFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return "log " + path
	}
	const max = 800
	s := string(b)
	if len(s) > max {
		s = s[len(s)-max:]
	}
	return "log " + path + ": " + strings.TrimSpace(s)
}

func defaultRestart(appDir string) func(appDir, binPath string) error {
	return func(dir, binPath string) error {
		return restartBinary(dir, binPath)
	}
}

func restartBinary(appDir, binPath string) error {
	runDir := filepath.Join(appDir, ".cogs", "run")
	pidPath := filepath.Join(runDir, "app.pid")
	if b, err := os.ReadFile(pidPath); err == nil {
		var pid int
		fmt.Sscanf(string(b), "%d", &pid)
		if pid > 0 {
			proc, _ := os.FindProcess(pid)
			if proc != nil {
				_ = proc.Kill()
			}
		}
		_ = os.Remove(pidPath)
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(runDir, "app.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(binPath, "serve", "-env", ".env")
	cmd.Dir = appDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = procspawn.Detached()
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	_ = logFile.Close()
	return os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)), 0o644)
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// RecordBuildFailure logs a failed Workers build without touching the backend.
func RecordBuildFailure(wsRoot, appID, commit, errMsg string) error {
	ev := cockpit.DeployEvent{
		App: appID, Action: "pull_build", Mode: appcfg.DeployPull,
		Status: "failed", GitCommit: commit, Detail: errMsg,
	}
	return cockpit.RecordDeploy(wsRoot, ev)
}

// RecordBuildSuccess triggers backend deploy after a successful Workers build.
func RecordBuildSuccess(ctx context.Context, wsRoot, appID, commit string) (string, error) {
	return DeployBackend(ctx, wsRoot, Options{
		AppID: appID, Commit: commit, Mode: appcfg.DeployPull,
		Action: "pull_build",
	})
}

// PlatformGO returns GOOS/GOARCH for the current deploy host.
func PlatformGO() (string, string) {
	return runtime.GOOS, runtime.GOARCH
}
