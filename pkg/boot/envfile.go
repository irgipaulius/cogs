package boot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpEnvPath is where daemonized cogs up reads COGS_DEPLOY_QUEUE_ID and similar.
func UpEnvPath(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", "boot", "cogs-up.env")
}

// WriteUpEnv merges keys into cogs-up.env (0o600).
func WriteUpEnv(wsRoot string, keys map[string]string) error {
	path := UpEnvPath(wsRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing := map[string]string{}
	if b, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if ok {
				existing[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	for k, v := range keys {
		if strings.TrimSpace(v) != "" {
			existing[k] = v
		}
	}
	var lines []string
	lines = append(lines, "# Written by CoGS — sourced by cogs up and boot scripts")
	for k, v := range existing {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	body := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(body), 0o600)
}
