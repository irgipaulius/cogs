package boot

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteLaunchdSourcesEnvNotHardcodedQueue(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd plist only on darwin")
	}
	dir := t.TempDir()
	ws := filepath.Join(dir, "repo")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(ws, "/usr/local/bin/cogs"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(ws, ".cogs", "boot", "com.cogs.up.plist"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if strings.Contains(body, "from cogs-up.env") {
		t.Fatalf("plist must not hardcode queue placeholder: %s", body)
	}
	if !strings.Contains(body, "cogs-up.env") {
		t.Fatalf("plist should source cogs-up.env: %s", body)
	}
}

func TestWriteRCRunsAsCogsUser(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, "repo")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(ws, "/usr/local/bin/cogs"); err != nil {
		t.Fatal(err)
	}
	var scriptPath string
	switch runtime.GOOS {
	case "freebsd":
		scriptPath = filepath.Join(ws, ".cogs", "boot", "cogs_up")
	default:
		// writeRC only on freebsd; synthesize same helper for assertion on all platforms
		if err := writeRC(filepath.Join(ws, ".cogs", "boot"), ws, "/usr/local/bin/cogs", UpEnvPath(ws)); err != nil {
			t.Fatal(err)
		}
		scriptPath = filepath.Join(ws, ".cogs", "boot", "cogs_up")
	}
	b, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "daemon -u cogs") {
		t.Fatalf("rc script must run daemon as cogs: %s", body)
	}
	if !strings.Contains(body, "cogs_up_status") || !strings.Contains(body, "kill -0") {
		t.Fatalf("rc script must implement status via pidfile: %s", body)
	}
}
