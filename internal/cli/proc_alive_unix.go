//go:build !windows

package cli

import "syscall"

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	var status syscall.WaitStatus
	n, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if err != nil || n > 0 {
		return false
	}
	return true
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
