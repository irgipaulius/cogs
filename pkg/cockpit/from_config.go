package cockpit

import (
	"path/filepath"

	"github.com/irgipaulius/cogs/pkg/config"
)

// FromConfig builds always-on telemetry for a CoGS app process.
func FromConfig(cfg config.App, appDir string) *Telemetry {
	name := cfg.AppName
	if name == "" {
		name = "app"
	}
	path := cfg.TelemetryPath
	if path == "" {
		path = filepath.Join(appDir, ".cogs", "run", "telemetry.json")
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(appDir, path)
	}
	return New(name, cfg.ListenAddr, cfg.PublicAPIURL, path)
}
