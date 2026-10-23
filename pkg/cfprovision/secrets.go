package cfprovision

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/fileutil"
	"github.com/irgipaulius/cogs/pkg/launch"
)

// EnsureEnv creates or updates .env for an app (preserves existing secrets).
func EnsureEnv(appDir string, m launch.Manifest, pageHost, apiHost string) error {
	path := filepath.Join(appDir, ".env")
	if _, err := os.Stat(path); err == nil {
		if err := setEnvKey(path, "COGS_LISTEN", listenAddr(m.BindHost, m.Port)); err != nil {
			return err
		}
		return appendEnvKeys(path, map[string]string{
			"COGS_PUBLIC_PAGE_ORIGIN": "https://" + pageHost,
			"COGS_PUBLIC_API_URL":     "https://" + apiHost,
			"COGS_OBS":                "file",
			"COGS_BLOB_STORE":         "fs",
			"COGS_BLOB_DIR":           "data/blobs",
		})
	}
	secret, _ := randomHex(32)
	content := fmt.Sprintf(`COGS_LISTEN=%s
COGS_DB_PATH=data/todo.db
COGS_SESSION_SECRET=%s
COGS_PUBLIC_PAGE_ORIGIN=https://%s
COGS_PUBLIC_API_URL=https://%s
COGS_BLOB_STORE=fs
COGS_BLOB_DIR=data/blobs
COGS_MAX_UPLOAD_BYTES=%d
COGS_R2_BUCKET=%s
COGS_OBS=file
`, listenAddr(m.BindHost, m.Port), secret, pageHost, apiHost, m.MaxUploadBytes, m.R2Bucket)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func listenAddr(host string, port int) string {
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 8080
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func setEnvKey(path, key, value string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	prefix := key + "="
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			if line == prefix+value {
				return nil
			}
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, prefix+value)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return fileutil.WriteFileIfChanged(path, []byte(out), 0o600)
}

func appendEnvKeys(path string, kv map[string]string) error {
	for k, v := range kv {
		if err := setEnvKey(path, k, v); err != nil {
			return err
		}
	}
	return nil
}
