package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/pkg/cfclient"
	"github.com/irgipaulius/cogs/pkg/cftoken"
	"github.com/irgipaulius/cogs/pkg/hostenv"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func tunnelRunDir(projectDir string) string {
	return filepath.Join(projectDir, ".cogs", "run")
}

func tunnelPIDPath(projectDir, name string) string {
	safe := strings.NewReplacer("/", "-", " ", "_").Replace(name)
	return filepath.Join(tunnelRunDir(projectDir), safe+".pid")
}

func tunnelLogPath(projectDir, name string) string {
	safe := strings.NewReplacer("/", "-", " ", "_").Replace(name)
	return filepath.Join(tunnelRunDir(projectDir), safe+".log")
}

func tunnelNameFromProject(projectDir string) (string, error) {
	m, err := launch.Load(projectDir)
	if err != nil {
		return "", err
	}
	return m.TunnelName, nil
}

func startTunnelBackgroundQuiet(projectDir, name string) error {
	return startTunnelBg(projectDir, name, true)
}

func startTunnelBackground(projectDir, name string) error {
	return startTunnelBg(projectDir, name, false)
}

func startTunnelBg(projectDir, name string, quiet bool) error {
	if projectDir == "" {
		projectDir = "."
	}
	if name == "" {
		var err error
		name, err = tunnelNameFromProject(projectDir)
		if err != nil {
			return fmt.Errorf("tunnel name required (or run from a project with cogs.launch.json)")
		}
	}
	key := "tunnel:" + name
	if !upGate.mayRetry(key) {
		return nil
	}
	if pid, running := tunnelRunning(projectDir, name); running {
		if !quiet {
			fmt.Printf("Tunnel %q already running (pid %d)\n", name, pid)
		}
		return nil
	}
	if holder, pid, held := tunnelHeldByOtherApp(projectDir, name); held {
		return fmt.Errorf("tunnel %q is already connected by app %q (pid %d) — each app needs its own tunnel_name in cogs.launch.json", name, holder, pid)
	}
	root, _, _ := workspace.FindRoot(projectDir)
	c := &cfclient.Client{WSRoot: root, OAuth: wranglerOAuthToken}
	ctx := context.Background()
	tok, err := c.Token(ctx, cftoken.ScopeAPI)
	if err != nil {
		upGate.recordAuthFailure(key, err.Error())
		return fmt.Errorf("Cloudflare auth missing: %w", err)
	}
	if err := os.MkdirAll(tunnelRunDir(projectDir), 0o755); err != nil {
		return err
	}
	logPath := tunnelLogPath(projectDir, name)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd, err := wranglerCmd("tunnel", "run", name)
	if err != nil {
		logFile.Close()
		return err
	}
	cmd.Env = append(hostenv.CLIEnv(root), "CLOUDFLARE_API_TOKEN="+tok)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		upGate.recordFailure(key, err.Error())
		return err
	}
	_ = logFile.Close()
	pidPath := tunnelPIDPath(projectDir, name)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	if !quiet {
		fmt.Printf("Tunnel %q started in background (pid %d)\n", name, cmd.Process.Pid)
		fmt.Printf("  log: %s\n", logPath)
		fmt.Printf("  stop: cogs tunnel stop %q --dir %s\n", name, projectDir)
	}
	return nil
}

func stopTunnel(projectDir, name string) error {
	if projectDir == "" {
		projectDir = "."
	}
	if name == "" {
		var err error
		name, err = tunnelNameFromProject(projectDir)
		if err != nil {
			return err
		}
	}
	pid, running := tunnelRunning(projectDir, name)
	if !running {
		fmt.Println("Tunnel not running")
		_ = os.Remove(tunnelPIDPath(projectDir, name))
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	_ = proc.Signal(syscall.SIGTERM)
	time.Sleep(500 * time.Millisecond)
	_ = os.Remove(tunnelPIDPath(projectDir, name))
	fmt.Printf("Stopped tunnel %q (pid %d)\n", name, pid)
	return nil
}

func tunnelHeldByOtherApp(projectDir, name string) (holder string, pid int, held bool) {
	root, ws, err := workspace.FindRoot(projectDir)
	if err != nil || name == "" {
		return "", 0, false
	}
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return "", 0, false
	}
	for _, a := range ws.Apps {
		dir := filepath.Join(root, a.Path)
		other, err := filepath.Abs(dir)
		if err != nil || other == abs {
			continue
		}
		m, err := launch.Load(dir)
		if err != nil || m.TunnelName != name {
			continue
		}
		if p, running := tunnelRunning(dir, name); running {
			return a.Name, p, true
		}
	}
	return "", 0, false
}

func tunnelRunning(projectDir, name string) (int, bool) {
	b, err := os.ReadFile(tunnelPIDPath(projectDir, name))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		_ = os.Remove(tunnelPIDPath(projectDir, name))
		return 0, false
	}
	return pid, true
}

func tunnelStatus(projectDir, name string) error {
	if projectDir == "" {
		projectDir = "."
	}
	if name == "" {
		var err error
		name, err = tunnelNameFromProject(projectDir)
		if err != nil {
			return err
		}
	}
	pid, ok := tunnelRunning(projectDir, name)
	if !ok {
		fmt.Printf("Tunnel %q: not running\n", name)
		return nil
	}
	fmt.Printf("Tunnel %q: running pid %d\n", name, pid)
	fmt.Printf("  log: %s\n", tunnelLogPath(projectDir, name))
	return nil
}

func tunnelRunForeground(name string) error {
	if name == "" {
		return fmt.Errorf("usage: cogs tunnel run <name>")
	}
	wd, _ := os.Getwd()
	root, _, _ := workspace.FindRoot(wd)
	c := &cfclient.Client{WSRoot: root, OAuth: wranglerOAuthToken}
	tok, err := c.Token(context.Background(), cftoken.ScopeAPI)
	if err != nil {
		return err
	}
	cmd, err := wranglerCmd("tunnel", "run", name)
	if err != nil {
		return err
	}
	cmd.Env = append(hostenv.CLIEnv(root), "CLOUDFLARE_API_TOKEN="+tok)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	fmt.Printf("Running: %s\n", formatWranglerCmd(cmd))
	return cmd.Run()
}

func formatWranglerCmd(cmd *exec.Cmd) string {
	return strings.Join(cmd.Args, " ")
}
