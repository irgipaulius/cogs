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
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/gowork"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/release"
	"github.com/irgipaulius/cogs/pkg/workspace"
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
	if !processAlive(pid) {
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
	return startAppProcess(projectDir, true, false)
}

func ensureAppRunningQuiet(projectDir string) error {
	return startAppProcess(projectDir, false, true)
}

func startAppProcess(projectDir string, rebuild, quietIfRunning bool) error {
	if pid, ok := appRunning(projectDir); ok {
		if !quietIfRunning {
			ux.AppOK(filepath.Base(projectDir), fmt.Sprintf("API already running pid %d", pid))
		}
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
	bin, err := appBinary(projectDir, rebuild)
	if err != nil {
		logFile.Close()
		return err
	}
	cmd := exec.Command(bin, "serve", "-env", ".env")
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
	markAppHealthGrace(projectDir)
	ux.AppOK(filepath.Base(projectDir), fmt.Sprintf("API started pid %d (%s)", cmd.Process.Pid, logPath))
	return nil
}

func appBinary(projectDir string, rebuild bool) (string, error) {
	combined := false
	if m, err := launch.Load(projectDir); err == nil {
		combined = m.IsCombined()
	}
	if wsRoot, ws, err := workspace.FindRoot(projectDir); err == nil {
		appID := workspace.CanonicalID(wsRoot, ws, projectDir)
		if p, ok := release.CurrentBinary(wsRoot, appID, combined); ok && !rebuild {
			return p, nil
		}
	}
	out := appBinaryPath(projectDir)
	if !rebuild {
		if st, err := os.Stat(out); err == nil && !st.IsDir() {
			return out, nil
		}
	}
	return buildAppBinary(projectDir)
}

func appBinaryPath(projectDir string) string {
	combined := false
	if app, err := appcfg.Load(projectDir); err == nil {
		combined = app.Kind == "combined"
	} else if m, err := launch.Load(projectDir); err == nil {
		combined = m.IsCombined()
	}
	name := release.BinaryName(combined)
	return filepath.Join(appRunDir(projectDir), name)
}

func buildAppBinary(projectDir string) (string, error) {
	out := appBinaryPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("go", "build", "-o", out, "./cmd/app")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if wsRoot, _, err := workspace.FindRoot(projectDir); err == nil {
		if env, e := gowork.BuildEnv(wsRoot, projectDir, os.Environ()); e == nil {
			cmd.Env = env
		}
	}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build app: %w", err)
	}
	return out, nil
}

func runAppBinary(projectDir string, args ...string) error {
	bin, err := buildAppBinary(projectDir)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
