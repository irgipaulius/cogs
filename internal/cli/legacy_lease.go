package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/irgipaulius/cogs/pkg/workspace"
)

// stopLegacyLeaseWatchers kills removed `cogs lease watch` helpers still referenced by old release binaries.
func stopLegacyLeaseWatchers(root string, ws workspace.Root) {
	for _, a := range ws.Apps {
		dir := filepath.Join(root, a.Path)
		pidPath := filepath.Join(dir, ".cogs", "run", "lease-watch.pid")
		b, err := os.ReadFile(pidPath)
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil || pid <= 0 {
			_ = os.Remove(pidPath)
			continue
		}
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
		_ = os.Remove(pidPath)
	}
}
