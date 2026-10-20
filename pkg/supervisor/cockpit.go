package supervisor

import (
	"path/filepath"

	"github.com/irgipaulius/cogs/pkg/cockpit"
)

func (r *MetricsRing) loadCockpit(appName, appDir string) cockpit.Snapshot {
	candidates := []string{
		filepath.Join(appDir, ".cogs", "run", "telemetry.json"),
		filepath.Join(appDir, "data", "telemetry.json"),
	}
	for _, p := range candidates {
		if s, err := cockpit.ReadFile(p); err == nil {
			return s
		}
	}
	return cockpit.Snapshot{App: appName}
}

func loadDeployMeta(wsRoot string) (map[string]cockpit.AppMeta, []cockpit.DeployEvent) {
	df, err := cockpit.LoadDeployments(wsRoot)
	if err != nil {
		return map[string]cockpit.AppMeta{}, nil
	}
	return df.Meta, df.Events
}
