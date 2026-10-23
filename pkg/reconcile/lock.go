package reconcile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func lockApp(wsRoot, appID string) (unlock func(), err error) {
	dir := filepath.Join(wsRoot, ".cogs", "locks")
	_ = os.MkdirAll(dir, 0o750)
	path := filepath.Join(dir, appID+".lock")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("reconcile lock timeout for %s", appID)
}
