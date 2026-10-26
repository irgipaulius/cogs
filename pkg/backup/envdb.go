package backup

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DBPathFromEnv reads COGS_DB_PATH or COGS_ADMIN_DB_PATH from app .env.
func DBPathFromEnv(appDir string) (string, error) {
	envPath := filepath.Join(appDir, ".env")
	f, err := os.Open(envPath)
	if err != nil {
		return "data.db", nil
	}
	defer f.Close()
	keys := []string{"COGS_DB_PATH", "COGS_ADMIN_DB_PATH"}
	found := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		found[k] = v
	}
	for _, k := range keys {
		if v := strings.TrimSpace(found[k]); v != "" {
			return v, nil
		}
	}
	return "data.db", nil
}
