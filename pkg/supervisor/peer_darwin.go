//go:build darwin

package supervisor

import "golang.org/x/sys/unix"

func peerPID(fd int) (int, error) {
	const localPeerPID = 0x002
	return unix.GetsockoptInt(fd, 0, localPeerPID)
}
