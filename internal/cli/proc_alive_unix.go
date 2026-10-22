//go:build !windows

package cli

import "syscall"

// processAlive reports whether pid exists (including processes not started by cogs up).
// Do not Wait4 arbitrary pids — that falsely marks launch/redeploy processes as dead.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func reapChildren() {
	for {
		var status syscall.WaitStatus
		n, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if err != nil || n <= 0 {
			return
		}
	}
}
