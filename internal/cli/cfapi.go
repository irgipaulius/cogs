package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type cfResponse struct {
	Success bool              `json:"success"`
	Errors  []cfError         `json:"errors"`
	Result  json.RawMessage   `json:"result"`
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// cfTokenDNS is for zone DNS records (COGS_CF_API_TOKEN). Wrangler OAuth cannot edit DNS.
func cfTokenDNS(projectDir string) (string, error) {
	if t := os.Getenv("COGS_CF_API_TOKEN"); t != "" {
		return t, nil
	}
	if projectDir != "" {
		if b, err := os.ReadFile(filepath.Join(projectDir, ".env")); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(line, "COGS_CF_API_TOKEN=") {
					t := strings.TrimSpace(strings.TrimPrefix(line, "COGS_CF_API_TOKEN="))
					t = strings.Trim(t, `"'`)
					if t != "" {
						return t, nil
					}
				}
			}
		}
	}
	return wranglerOAuthToken()
}

// cfTokenTunnel is for Cloudflare Tunnel / Pages API (Wrangler session). Do not use DNS-only tokens here.
func cfTokenTunnel() (string, error) {
	return wranglerOAuthToken()
}

// cfToken is an alias for DNS operations.
func cfToken(projectDir string) (string, error) {
	return cfTokenDNS(projectDir)
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

