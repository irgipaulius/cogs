package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func leaseWatchPIDPath(projectDir string) string {
	return filepath.Join(tunnelRunDir(projectDir), "lease-watch.pid")
}

func leaseWatchRunning(projectDir string) (int, bool) {
	b, err := os.ReadFile(leaseWatchPIDPath(projectDir))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		_ = os.Remove(leaseWatchPIDPath(projectDir))
		return 0, false
	}
	return pid, true
}

func spawnLeaseWatch(projectDir string) error {
	if err := os.MkdirAll(tunnelRunDir(projectDir), 0o755); err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	// When invoked via ./cogs wrapper, prefer dist/cogs for stable path.
	if strings.HasSuffix(bin, "cogs") && !strings.Contains(bin, string(os.PathSeparator)+"dist"+string(os.PathSeparator)) {
		root := filepath.Dir(bin)
		if alt := filepath.Join(root, "dist", "cogs"); alt != bin {
			if st, err := os.Stat(alt); err == nil && !st.IsDir() {
				bin = alt
			}
		}
	}
	logPath := filepath.Join(tunnelRunDir(projectDir), "lease-watch.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "lease", "watch", "--dir", projectDir)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	_ = logFile.Close()
	if err := os.WriteFile(leaseWatchPIDPath(projectDir), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	fmt.Printf("Lease watch started (pid %d) — stale hosts yield when a newer deploy claims %s\n", cmd.Process.Pid, logPath)
	return nil
}
