//go:build !windows

package procspawn

import "syscall"

// Detached sets process attributes for a background daemon on Unix.
func Detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
