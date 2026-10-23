package scraper

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveScriptPath returns an absolute path to the scraper script under appRoot.
func ResolveScriptPath(appRoot, configured string) string {
	appRoot = strings.TrimSpace(appRoot)
	if appRoot == "" {
		appRoot, _ = os.Getwd()
	}
	configured = strings.TrimSpace(configured)
	if configured == "" {
		configured = "scripts/scrape_chunk.py"
	}
	if filepath.IsAbs(configured) {
		return configured
	}
	// Avoid scripts/scripts when WORKDIR is already .../scripts.
	base := appRoot
	if wd := strings.TrimSpace(os.Getenv("COGS_WORKDIR")); wd != "" {
		base = wd
	}
	clean := filepath.Clean(configured)
	if strings.HasPrefix(clean, "..") {
		return filepath.Join(appRoot, clean)
	}
	candidate := filepath.Join(base, clean)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	alt := filepath.Join(appRoot, clean)
	if alt != candidate {
		if _, err := os.Stat(alt); err == nil {
			return alt
		}
	}
	return candidate
}
