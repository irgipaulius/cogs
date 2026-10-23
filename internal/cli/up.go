package cli

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/boot"
	"github.com/irgipaulius/cogs/pkg/cftoken"
	"github.com/irgipaulius/cogs/pkg/reconcile"
	"github.com/irgipaulius/cogs/pkg/state"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func cmdUp(args []string) error {
	foreground := false
	noTunnel := false
	for _, a := range args {
		switch a {
		case "--foreground", "-f":
			foreground = true
		case "--no-tunnel":
			noTunnel = true
		case "-h", "--help":
			return fmt.Errorf("usage: cogs up [--foreground] [--no-tunnel]")
		default:
			return fmt.Errorf("usage: cogs up [--foreground] [--no-tunnel]")
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, ws, err := workspace.FindRoot(wd)
	if err != nil {
		return fmt.Errorf("cogs up: %w", err)
	}
	boot.LoadUpEnvIntoProcess(root)
	if !foreground {
		return daemonizeUp(root)
	}
	return runUp(root, ws, noTunnel)
}

func cmdDown(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: cogs down")
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, ws, err := workspace.FindRoot(wd)
	if err != nil {
		return fmt.Errorf("cogs down: %w", err)
	}
	stopUpSupervisor(root)
	stopSupervisorBackground(root)
	stopWorkspaceApps(root, ws)
	ux.OK("stopped apps CoGS started")
	return nil
}

func upPIDPath(root string) string {
	return filepath.Join(root, ".cogs", "run", "up.pid")
}

func upLogPath(root string) string {
	return filepath.Join(root, ".cogs", "run", "up.log")
}

func daemonizeUp(root string) error {
	if pid, ok := appRunningPID(upPIDPath(root)); ok {
		ux.Info(fmt.Sprintf("cogs up already running pid %d", pid))
		return nil
	}
	if err := writeBootScript(root); err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := upLogPath(root)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(bin, "up", "--foreground")
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(upPIDPath(root), []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	fmt.Printf("cogs up running in background (pid %d)\n", cmd.Process.Pid)
	fmt.Printf("  log: %s\n", logPath)
	fmt.Printf("  boot: %s\n", bootScriptPath(root))
	fmt.Println("  stop: cogs down")
	return nil
}

func runUp(root string, ws workspace.Root, noTunnel bool) error {
	if err := writeBootScript(root); err != nil {
		return err
	}
	gcWranglerLogs(root)
	boot.LoadUpEnvIntoProcess(root)
	_ = os.Setenv("COGS_WORKSPACE_ROOT", root)
	cftoken.MaintainWranglerOAuth()
	attachUpLog(root)
	defer closeUpLog()
	_ = os.WriteFile(upPIDPath(root), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
	defer removePIDFileIfOwn(upPIDPath(root), os.Getpid())

	var st *state.Store
	var rec *reconcile.Manager
	if s, err := state.Open(root); err == nil {
		st = s
		rec = reconcile.NewManager(st)
		defer st.Close()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(5 * time.Second)
	reconcileTick := time.NewTicker(3 * time.Minute)
	oauthTick := time.NewTicker(10 * time.Minute)
	defer tick.Stop()
	defer reconcileTick.Stop()
	defer oauthTick.Stop()

	stopLegacyLeaseWatchers(root, ws)
	enqueueMonorepoRebuildIfNeeded(root, st, rec, ws)

	supervise := func(quiet bool) {
		reapChildren()
		rotateUpLogInProcess(root)
		ensureSupervisorRunning(root)
		for _, a := range ws.Apps {
			dir := filepath.Join(root, a.Path)
			if !appDirLooksLikeApp(dir) && st != nil {
				if v, err := st.GetApp(a.Name); err == nil && v.AppPath != "" {
					alt := filepath.Join(root, filepath.FromSlash(v.AppPath))
					if appDirLooksLikeApp(alt) {
						dir = alt
					}
				}
			}
			if !appDirLooksLikeApp(dir) {
				continue
			}
			if err := ensureReleaseRunningQuiet(dir); err != nil {
				upGate.recordFailure(a.Name, err.Error())
				if !quiet && upGate.shouldLog(a.Name, err.Error()) {
					ux.Warn(a.Name + ": " + err.Error())
				}
			} else if pid, ok := appRunning(dir); ok {
				noteAppStart(dir, pid)
			}
			recordAppCrashIfAny(st, a.Name, dir)
			recordAppBackgroundJobErrors(st, a.Name, dir)
			if noTunnel {
				continue
			}
			tKey := "tunnel:" + a.Name
			if err := startTunnelBackgroundQuiet(dir, ""); err != nil {
				msg := err.Error()
				if !quiet && upGate.shouldLog(tKey, msg) {
					log.Print(a.Name + " tunnel: " + msg)
				}
			}
		}
		upGate.writeAlerts(root)
		if st != nil {
			views, err := st.ListApps()
			if err == nil && len(views) == 0 && len(ws.Apps) > 0 {
				log.Printf("state.db has 0 apps but workspace has %d — re-open state or run cogs doctor", len(ws.Apps))
			}
		}
	}
	supervise(false)
	for {
		select {
		case <-sig:
			stopWorkspaceApps(root, ws)
			stopSupervisorBackground(root)
			return nil
		case <-tick.C:
			supervise(true)
			pullDeployQueueOnce(root, ws)
		case <-reconcileTick.C:
			if st != nil && rec != nil {
				views, _ := st.ListApps()
				for _, v := range views {
					if v.Present && !v.Converged {
						rec.Enqueue(v.ID, reconcile.Options{})
					}
				}
			}
		case <-oauthTick.C:
			cftoken.MaintainWranglerOAuth()
		}
	}
}

func stopUpSupervisor(root string) {
	pid, ok := appRunningPID(upPIDPath(root))
	if !ok {
		_ = os.Remove(upPIDPath(root))
		return
	}
	proc, err := os.FindProcess(pid)
	if err == nil {
		_ = proc.Signal(syscall.SIGTERM)
	}
	for i := 0; i < 20; i++ {
		if _, ok := appRunningPID(upPIDPath(root)); !ok {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func stopWorkspaceApps(root string, ws workspace.Root) {
	for _, a := range ws.Apps {
		dir := filepath.Join(root, a.Path)
		stopApp(dir)
		_ = stopTunnel(dir, "")
	}
}

func appRunningPID(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var pid int
	if _, err := fmt.Sscanf(string(b), "%d", &pid); err != nil || pid <= 0 {
		return 0, false
	}
	if !processAlive(pid) {
		return 0, false
	}
	return pid, true
}

func appDirLooksLikeApp(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "cogs.app.toml")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "cogs.launch.json")); err == nil {
		return true
	}
	return false
}

func bootScriptPath(root string) string {
	return filepath.Join(root, ".cogs", "boot", "cogs-up.sh")
}

func writeBootScript(root string) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	if err := boot.WriteShellScript(root, bin); err != nil {
		return err
	}
	return boot.Write(root, bin)
}

func removePIDFileIfOwn(pidPath string, pid int) {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	var got int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &got); err == nil && got == pid {
		_ = os.Remove(pidPath)
	}
}
