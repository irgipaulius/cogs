package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
)

func supervisorRunDir(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", "run")
}

func supervisorPIDPath(wsRoot string) string {
	return filepath.Join(supervisorRunDir(wsRoot), "supervisor.pid")
}

func startSupervisorBackground(wsRoot string) error {
	sock := filepath.Join(wsRoot, ".cogs", "supervisor.sock")
	if _, err := os.Stat(sock); err == nil {
		return nil
	}
	if err := os.MkdirAll(supervisorRunDir(wsRoot), 0o755); err != nil {
		return err
	}
	cogsBin, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := filepath.Join(supervisorRunDir(wsRoot), "supervisor.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(cogsBin, "supervisor", "run", "--workspace", wsRoot)
	cmd.Dir = wsRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	_ = logFile.Close()
	_ = os.WriteFile(supervisorPIDPath(wsRoot), []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644)
	// wait briefly for socket
	for i := 0; i < 30; i++ {
		if _, err := os.Stat(sock); err == nil {
			ux.Info("supervisor started")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}
