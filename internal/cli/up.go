package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/boot"
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

func rotateUpLog(root string) {
	const maxBytes = 2 << 20
	p := upLogPath(root)
	st, err := os.Stat(p)
	if err != nil || st.Size() < maxBytes {
		return
	}
	for i := 2; i >= 0; i-- {
		src := p
		if i > 0 {
			src = fmt.Sprintf("%s.%d", p, i)
		}
		dst := fmt.Sprintf("%s.%d", p, i+1)
		if i == 2 {
			_ = os.Remove(dst)
		}
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}
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
	rotateUpLog(root)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "up", "--foreground")
	cmd.Dir = root
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	_ = logFile.Close()
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
	boot.LoadUpEnvIntoProcess(root)
	rotateUpLog(root)
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
	defer tick.Stop()
	defer reconcileTick.Stop()

	supervise := func(quiet bool) {
		reapChildren()
		ensureSupervisorRunning(root)
		appDirs := map[string]string{}
		for _, a := range ws.Apps {
			appDirs[a.Name] = filepath.Join(root, a.Path)
		}
		if st != nil {
			views, err := st.ListApps()
			if err == nil {
				for _, v := range views {
					if !v.Present {
						continue
					}
					dir := appDirs[v.ID]
					if v.AppPath != "" {
						dir = filepath.Join(root, filepath.FromSlash(v.AppPath))
					}
					if dir == "" {
						if !v.Converged && rec != nil {
							rec.Enqueue(v.ID, reconcile.Options{})
						}
						continue
					}
					if _, err := os.Stat(filepath.Join(dir, "cogs.app.toml")); err != nil {
						if _, err2 := os.Stat(filepath.Join(dir, "cogs.launch.json")); err2 != nil {
							continue
						}
					}
					if err := ensureReleaseRunningQuiet(dir); err != nil {
						upGate.recordFailure(v.ID, err.Error())
						if !quiet && upGate.shouldLog(v.ID, err.Error()) {
							ux.Warn(v.ID + ": " + err.Error())
						}
					}
					if !v.Converged && rec != nil {
						rec.Enqueue(v.ID, reconcile.Options{})
					}
					if noTunnel {
						continue
					}
					if err := startTunnelBackgroundQuiet(dir, ""); err != nil {
						upGate.recordFailure(v.ID, err.Error())
					}
				}
				return
			}
		}
		for _, a := range ws.Apps {
			dir := filepath.Join(root, a.Path)
			if _, err := os.Stat(filepath.Join(dir, "cogs.launch.json")); err != nil {
				if _, err2 := os.Stat(filepath.Join(dir, "cogs.app.toml")); err2 != nil {
					continue
				}
			}
			if err := ensureReleaseRunningQuiet(dir); err != nil {
				upGate.recordFailure(a.Name, err.Error())
				if !quiet {
					ux.Warn(a.Name + ": " + err.Error())
				}
				continue
			}
			if noTunnel {
				continue
			}
			if err := startTunnelBackgroundQuiet(dir, ""); err != nil {
				upGate.recordFailure(a.Name, err.Error())
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
