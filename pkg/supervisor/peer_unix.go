//go:build !linux && !darwin && !windows && !freebsd

package supervisor

import "fmt"

func peerPID(fd int) (int, error) {
	return 0, fmt.Errorf("peer pid unsupported")
}

func peerSameUser(fd int) bool { return false }
