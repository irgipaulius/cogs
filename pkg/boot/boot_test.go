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
	for _, want := range []string{
		"/usr/sbin/daemon -f -u cogs --",      // plain daemon as cogs
		">> '$cogs_up_log' 2>&1",              // output via shell redirect
		"start|faststart|quietstart)",         // /etc/rc uses faststart at boot
		"while kill -0 \"$pid\"",              // stop waits for the old cogs up to exit
		"checkyesno cogs_up_enable || exit 0", // respects rcvar
		"onestart)",
		"stop|faststop|quietstop|onestop) cogs_up_stop",
		"restart|onerestart) cogs_up_stop; sleep 2; cogs_up_start",
		"cogs_up_status",
		"kill -0",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rc script missing %q:\n%s", want, body)
		}
	}
	// daemon supervisor flags crash with SIGSYS (kqueuex) on TrueNAS CORE hosts.
	for _, bad := range []string{"daemon -u cogs -p", "-o \"$cogs_up_log\"", "-p \"$pidfile\""} {
		if strings.Contains(body, bad) {
			t.Fatalf("rc script must not use %q:\n%s", bad, body)
		}
	}
}
