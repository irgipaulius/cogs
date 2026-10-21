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
	switch runtime.GOOS {
	case "windows":
		return writeWindows(dir, cogsBin)
	case "darwin":
		return writeLaunchd(dir, wsRoot, cogsBin)
	case "freebsd":
		return writeRC(dir, cogsBin)
	default:
		return writeSystemd(dir, wsRoot, cogsBin)
	}
}

func writeSystemd(dir, wsRoot, bin string) error {
	unit := fmt.Sprintf(`[Unit]
Description=CoGS supervisor (cogs up)
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%q up --foreground
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, wsRoot, bin)
	return os.WriteFile(filepath.Join(dir, "cogs-up.service"), []byte(unit), 0o644)
}

func writeRC(dir, bin string) error {
	body := fmt.Sprintf(`#!/bin/sh
# PROVIDER: cogs
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="cogs_up"
rcvar="cogs_up_enable"
command=%q
command_args="up --foreground"

load_rc_config $name
run_rc_command "$1"
`, bin)
	return os.WriteFile(filepath.Join(dir, "cogs_up"), []byte(body), 0o755)
}

func writeLaunchd(dir, wsRoot, bin string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.cogs.up</string>
  <key>ProgramArguments</key>
  <array><string>%s</string><string>up</string><string>--foreground</string></array>
  <key>WorkingDirectory</key><string>%s</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict></plist>
`, bin, wsRoot)
	return os.WriteFile(filepath.Join(dir, "com.cogs.up.plist"), []byte(plist), 0o644)
}

func writeWindows(dir, bin string) error {
	cmd := fmt.Sprintf(`schtasks /Create /TN "CoGS Up" /TR "\"%s\" up --foreground" /SC ONLOGON /RL HIGHEST /F
`, bin)
	return os.WriteFile(filepath.Join(dir, "install-scheduled-task.cmd"), []byte(cmd), 0o644)
}

// ShellScriptPath returns the legacy shell helper path (always written for Docker/FreeBSD cron).
func ShellScriptPath(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", "boot", "cogs-up.sh")
}

// WriteShellScript writes the portable exec wrapper used on FreeBSD cron and Docker.
func WriteShellScript(wsRoot, cogsBin string) error {
	path := ShellScriptPath(wsRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`#!/bin/sh
exec %q up --foreground
`, cogsBin)
	return os.WriteFile(path, []byte(body), 0o755)
}
