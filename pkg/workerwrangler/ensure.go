package workerwrangler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteGenerated writes assets-only wrangler.toml with custom domain routes (push/pull parity).
func WriteGenerated(projectDir, workerName, pageHost, pagesDir string) (genDir string, err error) {
	workerName = strings.TrimSpace(workerName)
	if workerName == "" {
		return "", fmt.Errorf("worker name required")
	}
	genDir = filepath.Join(projectDir, ".cogs", "generated", "worker")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return "", err
	}
	if pagesDir == "" {
		pagesDir = "public"
	}
	assetsDir, err := filepath.Abs(filepath.Join(projectDir, pagesDir))
	if err != nil {
		return "", err
	}
	routeBlock := ""
	if pageHost = strings.TrimSpace(pageHost); pageHost != "" {
		routeBlock = fmt.Sprintf(`
[[routes]]
pattern = %q
custom_domain = true
`, pageHost)
	}
	toml := fmt.Sprintf(`name = %q
compatibility_date = "2024-06-01"
workers_dev = false
%s
[assets]
directory = %q
`, workerName, routeBlock, assetsDir)
	path := filepath.Join(genDir, "wrangler.toml")
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		return "", err
	}
	return genDir, nil
}
