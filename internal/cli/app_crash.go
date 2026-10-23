package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/state"
)

// recordAppCrashIfAny sets last_error when the app process died and app.log shows a panic.
func recordAppCrashIfAny(st *state.Store, appID, projectDir string) {
	if st == nil || appID == "" {
		return
	}
	if _, ok := appRunning(projectDir); ok {
		return
	}
	msg := tailAppLogPanic(projectDir)
	if msg == "" {
		return
	}
	_ = st.SetConverged(appID, false, msg)
}

func tailAppLogPanic(projectDir string) string {
	logPath := filepath.Join(projectDir, ".cogs", "run", "app.log")
	b, err := os.ReadFile(logPath)
	if err != nil || len(b) == 0 {
		return ""
	}
	const tail = 8192
	if len(b) > tail {
		b = b[len(b)-tail:]
	}
	s := string(b)
	if !strings.Contains(s, "panic:") {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "panic:") {
			return "app crash: " + line
		}
	}
	return "app crash: panic in app.log"
}

func noteAppStart(projectDir string, pid int) {
	path := filepath.Join(projectDir, ".cogs", "run", "last-start")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano) + " " + strconv.Itoa(pid) + "\n"), 0o644)
}
