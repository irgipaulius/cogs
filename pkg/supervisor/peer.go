package supervisor

import (
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

// peerAllowed checks the unix socket peer is the admin HTTP process.
func peerAllowed(conn net.Conn) bool {
	if os.Getenv("COGS_ADMIN_DEV") == "1" {
		return true
	}
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return false
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return false
	}
	var pid int
	var credErr error
	_ = raw.Control(func(fd uintptr) {
		pid, credErr = peerPID(int(fd))
	})
	if credErr != nil || pid <= 0 {
		return false
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false
	}
	exe, err := proc.Exe()
	if err != nil {
		return exeFallback(pid)
	}
	return adminExecutableOK(exe)
}

func adminExecutableOK(exe string) bool {
	exe = filepath.Clean(exe)
	base := filepath.Base(exe)
	if base == "cogs-admin" || base == "app" {
		return strings.Contains(exe, string(filepath.Separator)+"apps"+string(filepath.Separator)+"admin"+string(filepath.Separator))
	}
	if strings.Contains(exe, "/apps/admin/") || strings.Contains(exe, `\apps\admin\`) {
		return true
	}
	if os.Getenv("COGS_ADMIN_DEV") == "1" {
		return strings.Contains(exe, "admin")
	}
	return false
}

func exeFallback(pid int) bool {
	return os.Getenv("COGS_ADMIN_DEV") == "1"
}
