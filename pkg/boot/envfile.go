package boot

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Keys allowed in cogs-up.env (supervisor-only; not passed to every app).
var upEnvAllowed = map[string]struct{}{
	"COGS_DEPLOY_QUEUE_ID": {},
	"COGS_CF_QUEUE_ID":     {},
}

// UpEnvPath is where daemonized cogs up reads COGS_DEPLOY_QUEUE_ID and similar.
func UpEnvPath(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", "boot", "cogs-up.env")
}

// WriteUpEnv merges keys into cogs-up.env (0o600). Only whitelisted keys are stored.
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
				k = strings.TrimSpace(k)
				if _, ok := upEnvAllowed[k]; ok {
					existing[k] = strings.TrimSpace(v)
				}
			}
		}
	}
	for k, v := range keys {
		if _, ok := upEnvAllowed[k]; !ok {
			continue
		}
		if strings.TrimSpace(v) != "" {
			existing[k] = v
		}
	}
	sorted := make([]string, 0, len(existing))
	for k := range existing {
		sorted = append(sorted, k)
	}
	slices.Sort(sorted)
	var lines []string
	lines = append(lines, "# Written by CoGS — sourced by cogs up and boot scripts (supervisor keys only)")
	for _, k := range sorted {
		lines = append(lines, fmt.Sprintf("%s=%s", k, existing[k]))
	}
	body := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(body), 0o600)
}

// LoadUpEnvIntoProcess sets whitelisted keys from cogs-up.env when unset in the environment.
func LoadUpEnvIntoProcess(wsRoot string) {
	path := UpEnvPath(wsRoot)
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if _, ok := upEnvAllowed[k]; !ok {
			continue
		}
		if k != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}
