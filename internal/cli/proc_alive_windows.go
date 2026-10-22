//go:build windows

package cli

import (
	"os"
	"syscall"
)

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	err = syscall.GetExitCodeProcess(h, &code)
	if err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}

func reapChildren() {}
