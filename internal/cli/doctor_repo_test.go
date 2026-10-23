package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorRepoJunkDetectsStrayFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "1"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := doctorRepoJunk(root)
	if len(findings) != 1 || findings[0].OK {
		t.Fatalf("findings=%+v", findings)
	}
}
