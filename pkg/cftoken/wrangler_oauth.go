package cftoken

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/irgipaulius/cogs/pkg/hostenv"
	"github.com/irgipaulius/cogs/pkg/wranglerpaths"
)

const wranglerOAuthClientID = "54d11594-84e4-41aa-b438-e81b8fa78ee7"

var oauthTokenURL = "https://dash.cloudflare.com/oauth2/token"

var wranglerRefreshMu sync.Mutex
var loggedConfigPath sync.Once
var wranglerNow = func() time.Time { return time.Now().UTC() }

// SetWranglerNowForTests replaces the clock used for OAuth expiry checks.
func SetWranglerNowForTests(fn func() time.Time) {
	if fn == nil {
		wranglerNow = func() time.Time { return time.Now().UTC() }
		return
	}
	wranglerNow = fn
}

type wranglerCreds struct {
	Path           string
	OAuthToken     string
	RefreshToken   string
	APIToken       string
	Expiration     time.Time
	HasExpiration  bool
}

// WranglerConfigPath returns wrangler default.toml with credentials.
func WranglerConfigPath() (string, error) {
	p, _, err := wranglerpaths.ResolveConfigDefaultTOML()
	return p, err
}

// WranglerOAuthToken returns a valid Cloudflare OAuth access token from wrangler config.
func WranglerOAuthToken() (string, error) {
	return wranglerOAuthToken(false)
}

func wranglerOAuthToken(forceRefresh bool) (string, error) {
	path, tried, err := wranglerpaths.ResolveConfigDefaultTOML()
	if err != nil {
		return "", formatNoTokenError("", tried, err)
	}
	loggedConfigPath.Do(func() {
		log.Printf("cftoken: wrangler config %s", path)
	})
	creds, err := readWranglerCreds(path)
	if err != nil {
		return "", formatNoTokenError(path, tried, err)
	}
	if creds.APIToken != "" && creds.OAuthToken == "" {
		return creds.APIToken, nil
	}
	if creds.OAuthToken == "" {
		return "", formatNoTokenError(path, tried, fmt.Errorf("no oauth_token or api_token in %s", path))
	}
	if forceRefresh || creds.needsRefresh() {
		if err := refreshWranglerOAuth(path); err != nil {
			if whoamiErr := refreshViaWranglerWhoami(); whoamiErr != nil {
				return "", formatNoTokenError(path, tried, fmt.Errorf("oauth refresh failed for %s: %v; wrangler whoami: %v", path, err, whoamiErr))
			}
		}
		creds, err = readWranglerCreds(path)
		if err != nil {
			return "", formatNoTokenError(path, tried, err)
		}
	}
	if creds.OAuthToken == "" {
		return "", formatNoTokenError(path, tried, fmt.Errorf("empty oauth_token after refresh"))
	}
	if creds.needsRefresh() {
		return "", formatNoTokenError(path, tried, fmt.Errorf("oauth_token expired at %s", creds.Expiration.UTC().Format(time.RFC3339)))
	}
	return creds.OAuthToken, nil
}

func (c wranglerCreds) needsRefresh() bool {
	if !c.HasExpiration {
		return false
	}
	return wranglerNow().After(c.Expiration.Add(-2 * time.Minute))
}

func readWranglerCreds(path string) (wranglerCreds, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return wranglerCreds{Path: path}, err
	}
	c := wranglerCreds{Path: path}
	c.OAuthToken = tomlStringField(b, "oauth_token")
	c.RefreshToken = tomlStringField(b, "refresh_token")
	c.APIToken = tomlStringField(b, "api_token")
	exp := tomlStringField(b, "expiration_time")
	if exp != "" {
		if t, err := time.Parse(time.RFC3339Nano, exp); err == nil {
			c.Expiration = t.UTC()
			c.HasExpiration = true
		} else if t, err := time.Parse(time.RFC3339, exp); err == nil {
			c.Expiration = t.UTC()
			c.HasExpiration = true
		}
	}
	return c, nil
}

func tomlStringField(b []byte, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=\s*"([^"]*)"`)
	m := re.FindSubmatch(b)
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}

func refreshWranglerOAuth(path string) error {
	wranglerRefreshMu.Lock()
	defer wranglerRefreshMu.Unlock()
	creds, err := readWranglerCreds(path)
	if err != nil {
		return err
	}
	if creds.RefreshToken == "" {
		return fmt.Errorf("no refresh_token in config")
	}
	body := url.Values{}
	body.Set("grant_type", "refresh_token")
	body.Set("refresh_token", creds.RefreshToken)
	body.Set("client_id", wranglerOAuthClientID)
	req, err := http.NewRequest(http.MethodPost, oauthTokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token endpoint HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return err
	}
	if tok.AccessToken == "" {
		return fmt.Errorf("empty access_token in refresh response")
	}
	exp := wranglerNow().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if err := patchWranglerConfig(path, tok.AccessToken, tok.RefreshToken, exp); err != nil {
		return err
	}
	return nil
}

func patchWranglerConfig(path, oauth, refresh string, exp time.Time) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := patchField(b, "oauth_token", oauth)
	if refresh != "" {
		out = patchField(out, "refresh_token", refresh)
	}
	out = patchField(out, "expiration_time", exp.Format("2006-01-02T15:04:05.000Z"))
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func patchField(b []byte, key, val string) []byte {
	line := key + ` = "` + val + `"` + "\n"
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=\s*".*"\s*\n?`)
	if re.Match(b) {
		return re.ReplaceAll(b, []byte(line))
	}
	out := append(bytes.TrimRight(b, "\n"), '\n')
	return append(out, []byte(line)...)
}

func refreshViaWranglerWhoami() error {
	bin := "wrangler"
	if _, err := exec.LookPath(bin); err != nil {
		if _, err2 := exec.LookPath("npx"); err2 != nil {
			return err
		}
		bin = "npx"
	}
	var cmd *exec.Cmd
	if bin == "npx" {
		cmd = exec.Command("npx", "wrangler", "whoami")
	} else {
		cmd = exec.Command("wrangler", "whoami")
	}
	cmd.Env = wranglerCLIEnv()
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(combined.String()))
	}
	return nil
}

func wranglerCLIEnv() []string {
	env := os.Environ()
	if home := hostenv.ServiceHome(); home != "" {
		prefix := "HOME="
		out := make([]string, 0, len(env)+1)
		for _, e := range env {
			if strings.HasPrefix(e, prefix) {
				continue
			}
			out = append(out, e)
		}
		return append(out, prefix+home)
	}
	return env
}

// MaintainWranglerOAuth refreshes OAuth tokens when close to expiry (for long-running cogs up).
func MaintainWranglerOAuth() {
	_, _ = wranglerOAuthToken(false)
}

func formatNoTokenError(activePath string, tried []string, reason error) error {
	var b strings.Builder
	b.WriteString("no Cloudflare token from wrangler config")
	if activePath != "" {
		b.WriteString(" (active path: ")
		b.WriteString(activePath)
		b.WriteByte(')')
	}
	if len(tried) > 0 {
		b.WriteString("; checked: ")
		b.WriteString(strings.Join(tried, "; "))
	}
	if reason != nil {
		b.WriteString("; ")
		b.WriteString(reason.Error())
	}
	return fmt.Errorf("%s", b.String())
}
