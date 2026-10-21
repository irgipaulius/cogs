package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/deploy"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func deployStatus(wsRoot string) (json.RawMessage, error) {
	df, err := cockpit.LoadDeployments(wsRoot)
	if err != nil {
		return nil, err
	}
	type appStatus struct {
		Meta   cockpit.AppMeta       `json:"meta"`
		Events []cockpit.DeployEvent `json:"recent_events"`
	}
	out := map[string]appStatus{}
	for app, meta := range df.Meta {
		st := appStatus{Meta: meta}
		for i := len(df.Events) - 1; i >= 0 && len(st.Events) < 10; i-- {
			if df.Events[i].App == app {
				st.Events = append(st.Events, df.Events[i])
			}
		}
		out[app] = st
	}
	b, err := json.Marshal(out)
	return b, err
}

func rollbackApp(ctx context.Context, wsRoot, appName string) (string, error) {
	if !hasApp(wsRoot, appName) {
		return "", fmt.Errorf("unknown app %q", appName)
	}
	return deploy.RollbackBackend(ctx, wsRoot, appName, nil)
}

func redeployApp(ctx context.Context, wsRoot, appName string) (detail string, err error) {
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return "", err
	}
	appDir, err := workspace.ResolveApp(wsRoot, ws, appName)
	if err != nil {
		return "", err
	}
	cfg, err := appcfg.Load(appDir)
	if err != nil {
		return "", err
	}
	if cfg.DeployMode != appcfg.DeployPull && cfg.Kind != "combined" {
		if bin, e := exec.LookPath("cogs"); e == nil {
			cmd := exec.CommandContext(ctx, bin, "launch", appDir, "--quick", "-y", "--no-serve")
			cmd.Dir = wsRoot
			out, err := cmd.CombinedOutput()
			detail = string(out)
			if err != nil {
				return detail, fmt.Errorf("frontend redeploy: %w", err)
			}
		}
	}
	if cfg.Kind == "combined" {
		if bin, e := exec.LookPath("cogs"); e == nil {
			cmd := exec.CommandContext(ctx, bin, "launch", appDir, "--quick", "-y", "--no-serve", "--skip-pages")
			cmd.Dir = wsRoot
			out, _ := cmd.CombinedOutput()
			detail = string(out)
			_ = cmd.Run()
		}
	}
	return deploy.DeployBackend(ctx, wsRoot, deploy.Options{
		AppID: appName, Mode: cfg.DeployMode, Action: "redeploy",
	})
}

func hasApp(wsRoot, name string) bool {
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return false
	}
	_, err = workspace.ResolveApp(wsRoot, ws, name)
	return err == nil
}
