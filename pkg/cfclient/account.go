package cfclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cftoken"
	"github.com/irgipaulius/cogs/pkg/hostenv"
	"github.com/irgipaulius/cogs/pkg/wranglercli"
)

const accountIDFile = ".cogs/cloudflare/account_id"

// AccountID returns the active Cloudflare account id.
func (c *Client) AccountID(ctx context.Context) (string, error) {
	if id := strings.TrimSpace(os.Getenv("COGS_CF_ACCOUNT_ID")); id != "" {
		return id, nil
	}
	if c.WSRoot != "" {
		if id, err := readStoredAccountID(c.WSRoot); err == nil && id != "" {
			return id, nil
		}
		if id := accountIDFromAppEnvs(c.WSRoot); id != "" {
			_ = storeAccountID(c.WSRoot, id)
			return id, nil
		}
	}
	if c.WSRoot != "" {
		_ = os.Setenv("COGS_WORKSPACE_ROOT", c.WSRoot)
	}
	if st, err := wranglercli.Status(ctx); err == nil && strings.TrimSpace(st.AccountID) != "" {
		id := st.AccountID
		if c.WSRoot != "" {
			_ = storeAccountID(c.WSRoot, id)
		}
		return id, nil
	}
	if id, err := accountIDFromWranglerConfig(); err == nil && id != "" {
		if c.WSRoot != "" {
			_ = storeAccountID(c.WSRoot, id)
		}
		return id, nil
	}
	if c.WSRoot != "" {
		_ = os.Setenv("COGS_WORKSPACE_ROOT", c.WSRoot)
	}
	tok, err := c.Token(ctx, cftoken.ScopeAPI)
	if err != nil {
		return "", fmt.Errorf("cloudflare account: %w", err)
	}
	id, err := accountIDFromToken(ctx, tok)
	if err != nil {
		return "", err
	}
	if c.WSRoot != "" {
		_ = storeAccountID(c.WSRoot, id)
	}
	return id, nil
}

func accountIDFromToken(ctx context.Context, tok string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.cloudflare.com/client/v4/accounts?per_page=50", nil)
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
			ID string `json:"id"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("accounts parse: %s", strings.TrimSpace(string(raw)))
	}
	if out.Success && len(out.Result) > 0 {
		return out.Result[0].ID, nil
	}
	// Token verify (works for some restricted tokens).
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.cloudflare.com/client/v4/user/tokens/verify", nil)
	req2.Header.Set("Authorization", "Bearer "+tok)
	resp2, err := http.DefaultClient.Do(req2)
	if err == nil {
		defer resp2.Body.Close()
		raw2, _ := io.ReadAll(resp2.Body)
		var ver struct {
			Success bool `json:"success"`
			Result  struct {
				Status string `json:"status"`
			} `json:"result"`
		}
		_ = json.Unmarshal(raw2, &ver)
	}
	msg := "no Cloudflare account for token (set COGS_CF_ACCOUNT_ID or use a token with Account read)"
	if len(out.Errors) > 0 {
		msg = out.Errors[0].Message
	}
	return "", fmt.Errorf("%s", msg)
}

func readStoredAccountID(wsRoot string) (string, error) {
	b, err := os.ReadFile(filepath.Join(wsRoot, accountIDFile))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func storeAccountID(wsRoot, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	dir := filepath.Join(wsRoot, ".cogs", "cloudflare")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "account_id"), []byte(id+"\n"), 0o644)
}

func accountIDFromWranglerConfig() (string, error) {
	path, err := cftoken.WranglerConfigPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?m)^account_id\s*=\s*"([^"]+)"`)
	if m := re.FindSubmatch(b); len(m) >= 2 {
		return string(m[1]), nil
	}
	return "", fmt.Errorf("no account_id in wrangler config")
}

func accountIDFromAppEnvs(wsRoot string) string {
	entries, err := os.ReadDir(filepath.Join(wsRoot, "apps"))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if id := parseEnvAccountID(filepath.Join(wsRoot, "apps", e.Name(), ".env")); id != "" {
			return id
		}
	}
	return ""
}

func parseEnvAccountID(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	keys := []string{"COGS_CF_ACCOUNT_ID", "COGS_R2_ACCOUNT_ID", "COGS_D1_ACCOUNT_ID"}
	for _, line := range strings.Split(string(b), "\n") {
		for _, k := range keys {
			if strings.HasPrefix(line, k+"=") {
				v := strings.TrimSpace(strings.TrimPrefix(line, k+"="))
				v = strings.Trim(v, `"'`)
				if v != "" {
					return v
				}
			}
		}
	}
	return ""
}

// CloudflareAuthStatus reports token availability for the dashboard.
func CloudflareAuthStatus(ctx context.Context, wsRoot string) map[string]any {
	out := map[string]any{
		"api_token_ok": false,
		"message":      "",
	}
	c := &Client{WSRoot: wsRoot}
	if wsRoot != "" {
		_ = os.Setenv("COGS_WORKSPACE_ROOT", wsRoot)
		home := hostenv.ServiceHome()
		if home != "" {
			_ = os.Setenv("HOME", home)
		}
	}
	if _, err := c.Token(ctx, cftoken.ScopeAPI); err != nil {
		out["message"] = "Cloudflare auth missing — set COGS_CF_API_TOKEN or run wrangler login"
		return out
	}
	out["api_token_ok"] = true
	if id, err := c.AccountID(ctx); err == nil {
		out["account_id"] = id
	} else {
		out["message"] = err.Error()
	}
	return out
}
