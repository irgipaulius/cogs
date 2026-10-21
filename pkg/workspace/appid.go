package workspace

import (
	"path/filepath"

	"github.com/irgipaulius/cogs/pkg/appcfg"
)

// CanonicalID returns the workspace app name for a project directory.
// Deploy history and the dashboard use this id, not manifest internal names like todo-app.
func CanonicalID(wsRoot string, ws Root, appDir string) string {
	abs, _ := filepath.Abs(appDir)
	for _, a := range ws.Apps {
		p, err := filepath.Abs(filepath.Join(wsRoot, a.Path))
		if err == nil && p == abs {
			return a.Name
		}
	}
	if app, err := appcfg.Load(appDir); err == nil && app.Name != "" {
		return app.Name
	}
	return filepath.Base(appDir)
}
