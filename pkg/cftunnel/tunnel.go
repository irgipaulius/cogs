package cftunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cfclient"
	"github.com/irgipaulius/cogs/pkg/cftoken"
)

// FindID resolves a tunnel UUID by name.
func FindID(ctx context.Context, c *cfclient.Client, accountID, name string) (string, error) {
	q := url.Values{}
	q.Set("name", name)
	q.Set("is_deleted", "false")
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel?%s", accountID, q.Encode())
	out, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	var tl struct {
		Result []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Result, &tl.Result); err != nil {
		return "", err
	}
	for _, t := range tl.Result {
		if t.Name == name {
			return t.ID, nil
		}
	}
	if len(tl.Result) == 1 {
		return tl.Result[0].ID, nil
	}
	return "", fmt.Errorf("tunnel %q not found", name)
}

// RunToken fetches a short-lived tunnel run token from the Cloudflare API.
func RunToken(ctx context.Context, c *cfclient.Client, accountID, tunnelID string) (string, error) {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/token", accountID, tunnelID)
	out, err := c.Do(ctx, cftoken.ScopeAPI, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	var raw string
	if err := json.Unmarshal(out.Result, &raw); err == nil && raw != "" {
		return raw, nil
	}
	var obj struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(out.Result, &obj); err != nil {
		return "", fmt.Errorf("tunnel token parse")
	}
	if obj.Token == "" {
		return "", fmt.Errorf("empty tunnel token")
	}
	return obj.Token, nil
}

// StartCloudflared runs cloudflared with an API-issued tunnel token (FreeBSD-safe).
func StartCloudflared(token, logPath string) (*exec.Cmd, error) {
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "tunnel", "run", "--token", token)
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		cmd.Stdout = f
		cmd.Stderr = f
	}
	cmd.Env = os.Environ()
	return cmd, nil
}

// WriteTokenFile stores token for debugging (0600).
func WriteTokenFile(dir, name, token string) (string, error) {
	_ = os.MkdirAll(dir, 0o750)
	safe := strings.NewReplacer("/", "-", " ", "_").Replace(name)
	p := filepath.Join(dir, safe+".token")
	if err := os.WriteFile(p, []byte(token), 0o600); err != nil {
		return "", err
	}
	return p, nil
}
