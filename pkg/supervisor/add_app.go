package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/irgipaulius/cogs/pkg/appadd"
)

func addAppFromRequest(ctx context.Context, wsRoot string, raw json.RawMessage) (string, error) {
	var body struct {
		Name           string `json:"name"`
		Repo           string `json:"repo"`
		DeployMode     string `json:"deploy_mode"`
		PageSubdomain  string `json:"page_subdomain"`
		APISubdomain   string `json:"api_subdomain"`
		Zone           string `json:"zone"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", err
	}
	detail, err := appadd.Add(appadd.Options{
		WSRoot:        wsRoot,
		Name:          body.Name,
		GitRepo:       body.Repo,
		DeployMode:    body.DeployMode,
		PageSubdomain: body.PageSubdomain,
		APISubdomain:  body.APISubdomain,
		Zone:          body.Zone,
	})
	if err != nil {
		return "", err
	}
	if bin, e := exec.LookPath("cogs"); e == nil {
		appDir, _ := appadd.AppDir(wsRoot, body.Name)
		cmd := exec.CommandContext(ctx, bin, "launch", appDir, "-y", "--no-serve")
		cmd.Dir = wsRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			return detail, fmt.Errorf("launch: %w: %s", err, string(out))
		}
	}
	return detail, nil
}
