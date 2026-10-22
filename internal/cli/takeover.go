package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/runner"
)

func claimDeployRunner(projectDir string) (string, string, error) {
	id, err := runner.EnsureID(projectDir)
	if err != nil {
		return "", "", err
	}
	at := runner.ClaimTime()
	envPath := projectDir + "/.env"
	if err := setEnvKey(envPath, "COGS_RUNNER_ID", id); err != nil {
		return "", "", err
	}
	if err := setEnvKey(envPath, "COGS_RUNNER_CLAIMED_AT", at); err != nil {
		return "", "", err
	}
	return id, at, nil
}

func disconnectAllTunnelConnectors(tunnelName string) error {
	accountID, err := cloudflareAccountID()
	if err != nil {
		return err
	}
	token, err := cfTokenTunnel()
	if err != nil {
		return err
	}
	tunnelID, err := findTunnelID(accountID, token, tunnelName)
	if err != nil {
		return err
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/connections", accountID, tunnelID)
	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Success bool `json:"success"`
	}
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode >= 300 || !out.Success {
		return fmt.Errorf("disconnect tunnel connectors: %s", string(raw))
	}
	return nil
}

func applyDeployTakeover(projectDir string, m launch.Manifest) error {
	id, at, err := claimDeployRunner(projectDir)
	if err != nil {
		return err
	}
	ux.Step(fmt.Sprintf("deploy claim runner %s at %s", id, at))
	if err := disconnectAllTunnelConnectors(m.TunnelName); err != nil {
		ux.Warn("tunnel disconnect (need Cloudflare Tunnel Write on API token): " + err.Error())
	} else {
		ux.OK("cleared remote tunnel connectors")
	}
	_ = stopTunnel(projectDir, m.TunnelName)
	stopApp(projectDir)
	return nil
}
