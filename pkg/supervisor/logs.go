package supervisor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

// TailLog returns the last maxLines from an app log (process or HTTP access).
func TailLog(appDir, kind string, maxLines int) ([]string, error) {
	if maxLines <= 0 || maxLines > 2000 {
		maxLines = 400
	}
	p := logPath(appDir, kind)
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("log: %w", err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > maxLines {
			lines = lines[1:]
		}
	}
	return lines, sc.Err()
}

func logPath(appDir, kind string) string {
	if kind == "access" {
		candidates := []string{
			filepath.Join(appDir, "data", "access.log"),
			filepath.Join(appDir, ".cogs", "run", "access.log"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return candidates[0]
	}
	return filepath.Join(appDir, ".cogs", "run", "app.log")
}
