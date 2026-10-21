package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/boot"
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
	stopWorkspaceApps(root, ws)
	ux.OK("stopped apps CoGS started")
	return nil
}

func upPIDPath(root string) string {
	return filepath.Join(root, ".cogs", "run", "up.pid")
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
	logPath := filepath.Join(root, ".cogs", "run", "up.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
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
	_ = os.WriteFile(upPIDPath(root), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
	defer os.Remove(upPIDPath(root))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	supervise := func() {
		for _, a := range ws.Apps {
			dir := filepath.Join(root, a.Path)
			if _, err := os.Stat(filepath.Join(dir, "cogs.launch.json")); err != nil {
				if _, err2 := os.Stat(filepath.Join(dir, "cogs.app.toml")); err2 != nil {
					continue
				}
			}
			if err := ensureAppRunning(dir); err != nil {
				ux.Warn(a.Name + ": " + err.Error())
			}
			if noTunnel {
				continue
			}
			if err := startTunnelBackground(dir, ""); err != nil {
				ux.Warn(a.Name + " tunnel: " + err.Error())
			}
		}
	}
	supervise()
	for {
		select {
		case <-sig:
			stopWorkspaceApps(root, ws)
			return nil
		case <-tick.C:
			supervise()
			pullDeployQueueOnce(root, ws)
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
	if err := syscall.Kill(pid, 0); err != nil {
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
