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
		return "", tunnelTokenErr(err)
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

func tunnelTokenErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "forbidden") || strings.Contains(msg, "10003") {
		return fmt.Errorf("Cloudflare API token lacks cfd_tunnel token permission — grant Account → Cloudflare Tunnel → Read (or Edit): %s", msg)
	}
	if strings.Contains(strings.ToLower(msg), "auth") {
		return fmt.Errorf("Cloudflare API token cannot read tunnel token — check token scopes: %s", msg)
	}
	return err
}

// StartCloudflared runs cloudflared with TUNNEL_TOKEN (not visible in argv).
func StartCloudflared(token, logPath string) (*exec.Cmd, error) {
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "tunnel", "run")
	cmd.Env = append(os.Environ(), "TUNNEL_TOKEN="+token)
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		cmd.Stdout = f
		cmd.Stderr = f
	}
	return cmd, nil
}

// StartCloudflaredWithFile runs cloudflared using --token-file (0600).
func StartCloudflaredWithFile(tokenPath, logPath string) (*exec.Cmd, error) {
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, "tunnel", "run", "--token-file", tokenPath)
	cmd.Env = os.Environ()
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		cmd.Stdout = f
		cmd.Stderr = f
	}
	return cmd, nil
}

// WriteTokenFile stores token for cloudflared (0600).
func WriteTokenFile(dir, name, token string) (string, error) {
	_ = os.MkdirAll(dir, 0o750)
	safe := strings.NewReplacer("/", "-", " ", "_").Replace(name)
	p := filepath.Join(dir, safe+".token")
	if err := os.WriteFile(p, []byte(token), 0o600); err != nil {
		return "", err
	}
	return p, nil
}
