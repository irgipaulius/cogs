// Package boot writes OS-specific helpers so cogs up survives reboot.
package boot

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Write installs a boot helper under .cogs/boot/ for the current OS.
func Write(wsRoot, cogsBin string) error {
	dir := filepath.Join(wsRoot, ".cogs", "boot")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	envFile := UpEnvPath(wsRoot)
	switch runtime.GOOS {
	case "windows":
		return writeWindows(dir, cogsBin, envFile)
	case "darwin":
		return writeLaunchd(dir, wsRoot, cogsBin, envFile)
	case "freebsd":
		return writeRC(dir, wsRoot, cogsBin, envFile)
	default:
		return writeSystemd(dir, wsRoot, cogsBin, envFile)
	}
}

func writeSystemd(dir, wsRoot, bin, envFile string) error {
	unit := fmt.Sprintf(`[Unit]
Description=CoGS supervisor (cogs up)
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%s
EnvironmentFile=-%s
ExecStart=%q up --foreground
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, wsRoot, envFile, bin)
	return os.WriteFile(filepath.Join(dir, "cogs-up.service"), []byte(unit), 0o644)
}

func writeRC(dir, wsRoot, bin, envFile string) error {
	pidFile := filepath.Join(wsRoot, ".cogs", "run", "up.pid")
	logFile := filepath.Join(wsRoot, ".cogs", "run", "up.log")
	body := fmt.Sprintf(`#!/bin/sh
# PROVIDE: cogs_up
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="cogs_up"
rcvar="cogs_up_enable"
pidfile=%q
command="/usr/sbin/daemon"
daemon_user="cogs"
daemon_flags="-p ${pidfile} -o ${pidfile}.log -f"
if [ -f %q ]; then
  set -a
  . %q
  set +a
fi
command_args="${daemon_flags} -- %q up --foreground"

load_rc_config $name
run_rc_command "$1"
`, pidFile, envFile, envFile, bin)
	_ = logFile
	return os.WriteFile(filepath.Join(dir, "cogs_up"), []byte(body), 0o755)
}

func writeLaunchd(dir, wsRoot, bin, envFile string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.cogs.up</string>
  <key>ProgramArguments</key>
  <array><string>%s</string><string>up</string><string>--foreground</string></array>
  <key>WorkingDirectory</key><string>%s</string>
  <key>EnvironmentVariables</key>
  <dict><key>COGS_DEPLOY_QUEUE_ID</key><string>from cogs-up.env</string></dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict></plist>
`, bin, wsRoot)
	_ = envFile
	return os.WriteFile(filepath.Join(dir, "com.cogs.up.plist"), []byte(plist), 0o644)
}

func writeWindows(dir, bin, envFile string) error {
	cmd := fmt.Sprintf(`@echo off
if exist %q for /f "usebackq tokens=*" %%a in (%q) do set %%a
"%s" up --foreground
`, envFile, envFile, bin)
	return os.WriteFile(filepath.Join(dir, "install-scheduled-task.cmd"), []byte(cmd), 0o644)
}

func ShellScriptPath(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", "boot", "cogs-up.sh")
}

func WriteShellScript(wsRoot, cogsBin string) error {
	path := ShellScriptPath(wsRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	envFile := UpEnvPath(wsRoot)
	body := fmt.Sprintf(`#!/bin/sh
if [ -f %q ]; then set -a; . %q; set +a; fi
exec %q up --foreground
`, envFile, envFile, cogsBin)
	return os.WriteFile(path, []byte(body), 0o755)
}
