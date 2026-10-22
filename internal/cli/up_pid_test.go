package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemovePIDFileIfOwn(t *testing.T) {
	p := filepath.Join(t.TempDir(), "up.pid")
	_ = os.WriteFile(p, []byte("222\n"), 0o644)
	removePIDFileIfOwn(p, 111)
	if _, err := os.Stat(p); err != nil {
		t.Fatal("removed another instance's pidfile")
	}
	removePIDFileIfOwn(p, 222)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("own pidfile not removed")
	}
}
