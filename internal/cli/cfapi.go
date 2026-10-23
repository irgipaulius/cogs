package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cftoken"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

type cfResponse struct {
	Success bool            `json:"success"`
	Errors  []cfError       `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// cfTokenDNS is for zone DNS records. Prefers workspace token, then env, then legacy app .env.
func cfTokenDNS(projectDir, zoneName string) (string, error) {
	if t := strings.TrimSpace(os.Getenv("COGS_CF_API_TOKEN")); t != "" {
		return t, nil
	}
	if wsRoot, _, err := workspace.FindRoot(projectDir); err == nil {
		if zoneName != "" {
			if t, err := workspace.DNSTokenForZone(wsRoot, zoneName); err == nil && t != "" {
				return t, nil
			}
		} else if t, err := workspace.DNSTokenForZone(wsRoot, ""); err == nil && t != "" {
			return t, nil
		}
	}
	if t := legacyAppCFToken(projectDir); t != "" {
		return t, nil
	}
	return wranglerOAuthToken()
}

func legacyAppCFToken(projectDir string) string {
	if projectDir == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(projectDir, ".env"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "COGS_CF_API_TOKEN=") {
			t := strings.TrimSpace(strings.TrimPrefix(line, "COGS_CF_API_TOKEN="))
			t = strings.Trim(t, `"'`)
			return t
		}
	}
	return ""
}

func migrateLegacyTokenToWorkspace(projectDir, zoneName string) {
	wsRoot, _, err := workspace.FindRoot(projectDir)
	if err != nil || zoneName == "" {
		return
	}
	if t, err := workspace.DNSTokenForZone(wsRoot, zoneName); err == nil && t != "" {
		return
	}
	legacy := legacyAppCFToken(projectDir)
	if legacy == "" {
		return
	}
	if !tokenCanEditDNS(legacy, zoneName) {
		return
	}
	_ = workspace.SaveDNSToken(wsRoot, zoneName, legacy)
}

// cfTokenTunnel is for Cloudflare Tunnel / Pages API (Wrangler session). Do not use DNS-only tokens here.
func cfTokenTunnel() (string, error) {
	return wranglerOAuthToken()
}

// cfTokenQueue prefers a Cloudflare API token (Queues) and falls back to the Wrangler OAuth session.
func cfTokenQueue(projectDir string) (string, error) {
	wsRoot := projectDir
	if root, _, err := workspace.FindRoot(projectDir); err == nil {
		wsRoot = root
	}
	r := cftoken.Resolver{WSRoot: wsRoot, OAuth: wranglerOAuthToken}
	tok, err := r.For(context.Background(), cftoken.ScopeQueue)
	if err != nil {
		return "", fmt.Errorf("queue API needs COGS_CF_QUEUE_API_TOKEN or COGS_CF_API_TOKEN with Account → Queues → Read and Messages → Consume: %w", err)
	}
	return tok, nil
}

func cfRequest(token, method, url string, body any) (cfResponse, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return cfResponse{}, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return cfResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return cfResponse{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return cfResponse{}, err
	}
	var out cfResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return cfResponse{}, fmt.Errorf("cloudflare api: %s", string(raw))
	}
	if !out.Success {
		msg := "cloudflare api error"
		if len(out.Errors) > 0 {
			msg = out.Errors[0].Message
		}
		return out, fmt.Errorf("%s", msg)
	}
	return out, nil
}
