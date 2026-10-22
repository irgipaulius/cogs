package wranglercli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// Zone is a Cloudflare DNS zone on the logged-in account.
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type zonesResponse struct {
	Success bool   `json:"success"`
	Result  []Zone `json:"result"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ListZones returns zones for the wrangler OAuth account (empty if not logged in).
func ListZones(ctx context.Context) ([]Zone, error) {
	st, err := Status(ctx)
	if err != nil || !st.LoggedIn || st.AccountID == "" {
		return nil, nil
	}
	token, err := oauthTokenFromConfig()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?account.id=%s&per_page=50", st.AccountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var zr zonesResponse
	if err := json.Unmarshal(body, &zr); err != nil {
		return nil, err
	}
	if !zr.Success {
		msg := "zones API failed"
		if len(zr.Errors) > 0 {
			msg = zr.Errors[0].Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return zr.Result, nil
}

func oauthTokenFromConfig() (string, error) {
	path, err := wranglerConfigPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read wrangler config: %w", err)
	}
	re := regexp.MustCompile(`(?m)^oauth_token\s*=\s*"([^"]+)"`)
	m := re.FindSubmatch(b)
	if len(m) < 2 {
		re2 := regexp.MustCompile(`(?m)^api_token\s*=\s*"([^"]+)"`)
		m = re2.FindSubmatch(b)
	}
	if len(m) < 2 {
		return "", fmt.Errorf("no oauth_token; run wrangler login")
	}
	return string(m[1]), nil
}

func wranglerConfigPath() (string, error) {
	if h := os.Getenv("WRANGLER_HOME"); h != "" {
		return filepath.Join(h, "config", "default.toml"), nil
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Preferences", ".wrangler", "config", "default.toml"), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, ".wrangler", "config", "default.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", ".wrangler", "config", "default.toml"), nil
}
