package runner

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const idFile = "runner-id"

// EnsureID returns a stable runner id for this project directory.
func EnsureID(projectDir string) (string, error) {
	dir := filepath.Join(projectDir, ".cogs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, idFile)
	if b, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(b))
		if id != "" {
			return id, nil
		}
	}
	host, _ := os.Hostname()
	rnd := make([]byte, 4)
	_, _ = rand.Read(rnd)
	id := host + "-" + hex.EncodeToString(rnd)
	if err := os.WriteFile(path, []byte(id), 0o644); err != nil {
		return "", err
	}
	return id, nil
}

// ClaimTime is RFC3339 UTC for comparing deploy generations.
func ClaimTime() string {
	return time.Now().UTC().Format(time.RFC3339)
}
