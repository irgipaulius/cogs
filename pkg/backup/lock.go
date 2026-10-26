package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func lockPath(appDir string) string {
	return filepath.Join(appDir, ".cogs", "run", "backup.lock")
}

func acquireAppLock(appDir string) error {
	path := lockPath(appDir)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("backup lock timeout")
}

func releaseAppLock(appDir string) {
	_ = os.Remove(lockPath(appDir))
}
