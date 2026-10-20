package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
)

func appRunDir(projectDir string) string {
	return filepath.Join(projectDir, ".cogs", "run")
}

func appPIDPath(projectDir string) string {
	return filepath.Join(appRunDir(projectDir), "app.pid")
}

func appLogPath(projectDir string) string {
	return filepath.Join(appRunDir(projectDir), "app.log")
}

func appRunning(projectDir string) (int, bool) {
	b, err := os.ReadFile(appPIDPath(projectDir))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		_ = os.Remove(appPIDPath(projectDir))
		return 0, false
	}
	return pid, true
}

func stopApp(projectDir string) {
	if pid, ok := appRunning(projectDir); ok {
		proc, _ := os.FindProcess(pid)
		if proc != nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
		time.Sleep(300 * time.Millisecond)
		_ = os.Remove(appPIDPath(projectDir))
	}
}

func startAppBackground(projectDir string) error {
	if pid, ok := appRunning(projectDir); ok {
		ux.AppOK(filepath.Base(projectDir), fmt.Sprintf("API already running pid %d", pid))
		return nil
	}
	stopApp(projectDir)
	if err := os.MkdirAll(appRunDir(projectDir), 0o755); err != nil {
		return err
	}
	logPath := appLogPath(projectDir)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "run", "./cmd/app", "serve", "-env", ".env")
	cmd.Dir = projectDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	_ = logFile.Close()
	if err := os.WriteFile(appPIDPath(projectDir), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	ux.AppOK(filepath.Base(projectDir), fmt.Sprintf("API started pid %d (%s)", cmd.Process.Pid, logPath))
	return nil
}
