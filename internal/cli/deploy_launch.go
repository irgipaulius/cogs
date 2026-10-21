package cli

import (
	"context"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/deploy"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func deployBackendQuick(projectDir string, m launch.Manifest, action string) error {
	wsRoot, ws, err := workspace.FindRoot(projectDir)
	if err != nil {
		return err
	}
	appID := workspace.CanonicalID(wsRoot, ws, projectDir)
	mode := m.DeployMode
	if mode == "" {
		if app, err := appcfg.Load(projectDir); err == nil {
			mode = app.DeployMode
		}
	}
	_, err = deploy.DeployBackend(context.Background(), wsRoot, deploy.Options{
		AppID: appID, Mode: mode, Action: action,
	})
	return err
}
