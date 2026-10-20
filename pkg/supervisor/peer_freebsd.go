//go:build freebsd

package supervisor

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// peerPID is unused for the allow decision on FreeBSD: xucred in x/sys does not
// expose cr_pid. Identity is the socket peer uid (see peerSameUser).
func peerPID(fd int) (int, error) {
	return 0, fmt.Errorf("peer pid unsupported")
}

// peerSameUser reports whether the unix-socket caller has the same uid as this process.
func peerSameUser(fd int) bool {
	cred, err := unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	if err != nil {
		return false
	}
	return int(cred.Uid) == os.Getuid()
}
