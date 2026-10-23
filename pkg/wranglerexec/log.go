package wranglerexec

import (
	"os"
)

const workspaceWranglerLogMax = 512 << 10 // 512 KiB

// RotateWorkspaceLog truncates the workspace wrangler log when it grows too large.
func RotateWorkspaceLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= workspaceWranglerLogMax {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	keep := b
	if len(keep) > workspaceWranglerLogMax/2 {
		keep = keep[len(keep)-workspaceWranglerLogMax/2:]
	}
	_ = os.WriteFile(path, append([]byte("… truncated …\n"), keep...), 0o644)
}
