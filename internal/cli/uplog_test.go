package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpLogBytesSinceCurrentRun(t *testing.T) {
	root := t.TempDir()
	path := upLogPath(root)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	body := "queue pull token: error from old binary\ncogs up start pid=1 at=2026-01-01T00:00:00Z\nqueue pull: ok\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := string(upLogBytesSinceCurrentRun(root))
	if strings.Contains(got, "old binary") {
		t.Fatalf("included old run: %q", got)
	}
	if !strings.Contains(got, "queue pull: ok") {
		t.Fatalf("missing current run: %q", got)
	}
}
