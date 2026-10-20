//go:build windows

package supervisor

import "fmt"

func peerPID(fd int) (int, error) {
	return 0, fmt.Errorf("supervisor socket not supported on windows")
}
