//go:build windows

package procspawn

import "syscall"

// Detached starts the process in a new process group on Windows.
func Detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
