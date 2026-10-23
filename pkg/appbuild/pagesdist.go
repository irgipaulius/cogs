package appbuild

import (
	"path/filepath"
)

// PagesDistDir is the untracked UI build output under the app (.cogs/pages-dist).
func PagesDistDir(appDir string) string {
	return filepath.Join(appDir, ".cogs", "pages-dist")
}
