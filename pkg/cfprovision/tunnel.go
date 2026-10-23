package cfprovision

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cfclient"
	"github.com/irgipaulius/cogs/pkg/cftoken"
)

type ingressRule struct {
	Hostname string `json:"hostname,omitempty"`
	Service  string `json:"service"`
}

func ensureTunnelIngress(ctx context.Context, c *cfclient.Client, accountID, tunnelID, hostname, localService string) error {
	ingress, err := getTunnelIngress(ctx, c, accountID, tunnelID)
	if err != nil {
		ingress = nil
	}
	ingress = mergeIngress(ingress, hostname, localService)
	body := map[string]any{
		"config": map[string]any{"ingress": ingress},
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations", accountID, tunnelID)
	_, err = c.Do(ctx, cftoken.ScopeAPI, http.MethodPut, apiURL, body)
	return err
}

func getTunnelIngress(ctx context.Context, c *cfclient.Client, accountID, tunnelID string) ([]ingressRule, error) {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations", accountID, tunnelID)
	out, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	var cfg struct {
		Result struct {
			Config struct {
				Ingress []ingressRule `json:"ingress"`
			} `json:"config"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Result, &cfg.Result); err != nil {
		return nil, err
	}
	return cfg.Result.Config.Ingress, nil
}

func mergeIngress(existing []ingressRule, hostname, localService string) []ingressRule {
	var out []ingressRule
	replaced := false
	for _, r := range existing {
		if r.Hostname == hostname {
			out = append(out, ingressRule{Hostname: hostname, Service: localService})
			replaced = true
			continue
		}
		if r.Service == "http_status:404" {
			continue
		}
		out = append(out, r)
	}
	if !replaced {
		out = append(out, ingressRule{Hostname: hostname, Service: localService})
	}
	out = append(out, ingressRule{Service: "http_status:404"})
	return out
}

func deleteTunnel(ctx context.Context, c *cfclient.Client, accountID, tunnelID string) error {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s", accountID, tunnelID)
	_, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodDelete, apiURL, nil)
	return err
}

// tunnelCreateViaAPI creates a named tunnel when wrangler is unavailable.
func tunnelCreateViaAPI(ctx context.Context, c *cfclient.Client, accountID, name string) (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	body := map[string]any{
		"name":          name,
		"tunnel_secret": base64.StdEncoding.EncodeToString(secret),
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel", accountID)
	out, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodPost, apiURL, body)
	if err != nil {
		return "", err
	}
	var created struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Result, &created.Result); err != nil {
		return "", err
	}
	return created.Result.ID, nil
}

func tunnelTarget(tunnelID string) string {
	return strings.TrimSpace(tunnelID) + ".cfargotunnel.com"
}
