//go:build !linux && !darwin && !windows

package supervisor

import "fmt"

func peerPID(fd int) (int, error) {
	return 0, fmt.Errorf("peer pid unsupported")
}
