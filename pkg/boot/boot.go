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
# REQUIRE: NETWORKING DAEMON
# KEYWORD: shutdown

. /etc/rc.subr

name="cogs_up"
rcvar="cogs_up_enable"
pidfile=%q
cogs_up_log=%q
cogs_up_env=%q
cogs_up_bin=%q
cogs_up_root=%q

cogs_up_start() {
	if [ -f "$cogs_up_env" ]; then
		set -a
		# shellcheck disable=SC1090
		. "$cogs_up_env"
		set +a
	fi
	# Plain daemon -f: the supervisor mode (-p/-o/-r) of a 13.x jail userland calls
	# kqueuex, which older host kernels (TrueNAS CORE) lack, killing it with SIGSYS.
	# cogs up writes $pidfile itself.
	/usr/sbin/daemon -f -u cogs -- \
		/bin/sh -c "cd '$cogs_up_root' && exec '$cogs_up_bin' up --foreground >> '$cogs_up_log' 2>&1"
}

cogs_up_stop() {
	if [ -f "$pidfile" ]; then
		read -r pid < "$pidfile" 2>/dev/null || pid=
		if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
			kill "$pid" 2>/dev/null
			# wait(1) cannot wait for a non-child: poll until cogs up has
			# finished stopping its apps and tunnels (max 60s), so a following
			# start never races the old instance's shutdown.
			i=0
			while kill -0 "$pid" 2>/dev/null && [ $i -lt 120 ]; do
				sleep 0.5
				i=$((i + 1))
			done
		fi
		rm -f "$pidfile"
	fi
}

cogs_up_status() {
	if [ -f "$pidfile" ]; then
		read -r pid < "$pidfile" 2>/dev/null || pid=
		if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
			echo "cogs_up is running as pid $pid."
			return 0
		fi
	fi
	echo "cogs_up is not running."
	return 1
}

load_rc_config $name
: ${cogs_up_enable:="NO"}

# /etc/rc calls faststart at boot; honour cogs_up_enable except for one* verbs.
case "$1" in
start|faststart|quietstart)
	checkyesno cogs_up_enable || exit 0
	cogs_up_status >/dev/null 2>&1 && exit 0
	cogs_up_start ;;
onestart)
	cogs_up_status >/dev/null 2>&1 && exit 0
	cogs_up_start ;;
stop|faststop|quietstop|onestop) cogs_up_stop ;;
restart|onerestart) cogs_up_stop; sleep 2; cogs_up_start ;;
status|onestatus) cogs_up_status ;;
*) run_rc_command "$1" ;;
esac
`, pidFile, logFile, envFile, bin, wsRoot)
	return os.WriteFile(filepath.Join(dir, "cogs_up"), []byte(body), 0o755)
}

func writeLaunchd(dir, wsRoot, bin, envFile string) error {
	// Source cogs-up.env at runtime; do not hardcode queue id in the plist.
	runLine := fmt.Sprintf(
		"set -a; [ -f %q ] && . %q; set +a; cd %q; exec %q up --foreground",
		envFile, envFile, wsRoot, bin,
	)
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.cogs.up</string>
  <key>ProgramArguments</key>
  <array><string>/bin/sh</string><string>-c</string><string>%s</string></array>
  <key>WorkingDirectory</key><string>%s</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict></plist>
`, runLine, wsRoot)
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
