package appbuild

import (
	"os"
	"path/filepath"
)

// HasFrontend reports whether the app has a UI build (frontend/package.json).
func HasFrontend(appDir string) bool {
	_, err := os.Stat(filepath.Join(appDir, "frontend", "package.json"))
	return err == nil
}
