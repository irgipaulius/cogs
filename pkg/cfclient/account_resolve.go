package cfclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cftoken"
	"github.com/irgipaulius/cogs/pkg/launch"
)

func accountIDFromZones(ctx context.Context, tok string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.cloudflare.com/client/v4/zones?per_page=1", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Success bool `json:"success"`
		Result  []struct {
			ID        string `json:"id"`
			Account   struct {
				ID string `json:"id"`
			} `json:"account"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Success && len(out.Result) > 0 {
		if id := strings.TrimSpace(out.Result[0].Account.ID); id != "" {
			return id, nil
		}
	}
	msg := "zones API did not return account.id"
	if len(out.Errors) > 0 {
		msg = out.Errors[0].Message
	}
	return "", fmt.Errorf("%s", msg)
}

func accountIDFromTunnelToken(tok string) string {
	parts := strings.Split(strings.TrimSpace(tok), ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// padded fallback
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		AccountID string `json:"a"`
		Account   string `json:"account_id"`
		Sub       string `json:"sub"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	if id := strings.TrimSpace(claims.AccountID); id != "" {
		return id
	}
	if id := strings.TrimSpace(claims.Account); id != "" {
		return id
	}
	// sub is sometimes account uuid for tunnel tokens
	if id := strings.TrimSpace(claims.Sub); len(id) >= 32 && strings.Count(id, "-") >= 4 {
		return id
	}
	return ""
}

func accountIDFromWorkspaceApps(wsRoot string) string {
	entries, err := os.ReadDir(filepath.Join(wsRoot, "apps"))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(wsRoot, "apps", e.Name())
		if id := parseEnvAccountID(filepath.Join(dir, ".env")); id != "" {
			return id
		}
		if m, err := launch.Load(dir); err == nil {
			if id := accountIDFromLaunchEnv(dir, m); id != "" {
				return id
			}
		}
		if cfg, err := appcfg.Load(dir); err == nil && strings.TrimSpace(cfg.Zone) != "" {
			// zone alone does not give account id
			_ = cfg
		}
	}
	return ""
}

func accountIDFromLaunchEnv(appDir string, m launch.Manifest) string {
	_ = m
	return parseEnvAccountID(filepath.Join(appDir, ".env"))
}

func accountIDFromStoredTunnelToken(wsRoot string) string {
	runDir := filepath.Join(wsRoot, ".cogs", "run")
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".token") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(runDir, e.Name()))
		if err != nil {
			continue
		}
		if id := accountIDFromTunnelToken(string(b)); id != "" {
			return id
		}
	}
	return ""
}

// VerifyTunnelTokenAccess checks the API token can fetch a tunnel run token.
func (c *Client) VerifyTunnelTokenAccess(ctx context.Context, accountID, tunnelID string) error {
	if accountID == "" || tunnelID == "" {
		return fmt.Errorf("tunnel verify: missing account or tunnel id")
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/token", accountID, tunnelID)
	_, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodGet, apiURL, nil)
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "authentication") || strings.Contains(msg, "10000") {
		return fmt.Errorf("Cloudflare API token cannot read tunnel token — grant Account → Cloudflare Tunnel → Read (or Edit)")
	}
	if strings.Contains(strings.ToLower(msg), "forbidden") || strings.Contains(msg, "10003") {
		return fmt.Errorf("Cloudflare API token lacks permission for cfd_tunnel/%s/token — need Cloudflare Tunnel Read on this account", tunnelID)
	}
	return fmt.Errorf("tunnel token API: %s", msg)
}
