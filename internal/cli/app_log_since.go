package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func appLogBytesSinceStart(projectDir string) []byte {
	logPath := filepath.Join(projectDir, ".cogs", "run", "app.log")
	markPath := filepath.Join(projectDir, ".cogs", "run", "app.log.start")
	b, err := os.ReadFile(logPath)
	if err != nil || len(b) == 0 {
		return nil
	}
	off := int64(0)
	if m, err := os.ReadFile(markPath); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(string(m)), 10, 64); err == nil && n >= 0 && n <= int64(len(b)) {
			off = n
		}
	}
	return b[off:]
}

func writeAppLogStartMark(projectDir string) {
	logPath := appLogPath(projectDir)
	markPath := filepath.Join(appRunDir(projectDir), "app.log.start")
	var off int64
	if st, err := os.Stat(logPath); err == nil {
		off = st.Size()
	}
	_ = os.WriteFile(markPath, []byte(strconv.FormatInt(off, 10)+"\n"), 0o644)
}
