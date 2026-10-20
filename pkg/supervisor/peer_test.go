package supervisor

import (
	"path/filepath"
	"testing"
)

func TestAdminExecutableOKBuiltBinary(t *testing.T) {
	exe := filepath.Join("/var", "cogs", "apps", "admin", ".cogs", "run", "cogs-admin")
	if !adminExecutableOK(exe) {
		t.Fatalf("built admin binary should be allowed: %s", exe)
	}
	if adminExecutableOK(filepath.Join("/tmp", "go-build123", "exe", "cogs-admin")) {
		t.Fatal("go run temp binary must not pass the path check")
	}
}
