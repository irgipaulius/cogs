package githubauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	deviceCodeURL = "https://github.com/login/device/code"
	accessTokenURL = "https://github.com/login/oauth/access_token"
	apiUserURL     = "https://api.github.com/user"
)

// DeviceStart begins the OAuth device flow.
type DeviceStart struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// ClientID from COGS_GITHUB_CLIENT_ID (required for device flow).
func ClientID() (string, error) {
	id := strings.TrimSpace(os.Getenv("COGS_GITHUB_CLIENT_ID"))
	if id == "" {
		return "", fmt.Errorf("set COGS_GITHUB_CLIENT_ID to a GitHub OAuth App client id (public), then run cogs github login")
	}
	return id, nil
}

// StartDeviceFlow requests a user code.
func StartDeviceFlow(clientID string) (DeviceStart, error) {
	body := fmt.Sprintf("client_id=%s&scope=%s", clientID, "repo read:user")
	req, err := http.NewRequest(http.MethodPost, deviceCodeURL, strings.NewReader(body))
	if err != nil {
		return DeviceStart{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DeviceStart{}, err
	}
	defer resp.Body.Close()
	var start DeviceStart
	if err := json.NewDecoder(resp.Body).Decode(&start); err != nil {
		return DeviceStart{}, err
	}
	if start.DeviceCode == "" {
		return DeviceStart{}, fmt.Errorf("github device flow: empty device_code")
	}
	if start.VerificationURI == "" {
		start.VerificationURI = "https://github.com/login/device"
	}
	if start.Interval <= 0 {
		start.Interval = 5
	}
	return start, nil
}

// PollDeviceFlowOnce tries one token poll (for dashboard tick).
func PollDeviceFlowOnce(clientID, deviceCode string) (Credential, error) {
	body := fmt.Sprintf("client_id=%s&device_code=%s&grant_type=urn:ietf:params:oauth:grant-type:device_code",
		clientID, deviceCode)
	req, err := http.NewRequest(http.MethodPost, accessTokenURL, strings.NewReader(body))
	if err != nil {
		return Credential{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Credential{}, err
	}
	defer resp.Body.Close()
	var raw map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&raw)
	if errMsg, _ := raw["error"].(string); errMsg == "authorization_pending" {
		return Credential{}, fmt.Errorf("authorization_pending")
	}
	if errMsg, _ := raw["error"].(string); errMsg != "" && errMsg != "authorization_pending" {
		return Credential{}, fmt.Errorf("github: %s", errMsg)
	}
	tok, _ := raw["access_token"].(string)
	if tok == "" {
		return Credential{}, fmt.Errorf("authorization_pending")
	}
	scopes, _ := raw["scope"].(string)
	c := Credential{Token: tok, Scopes: scopes, CreatedAt: time.Now().UTC()}
	if login, err := fetchLogin(tok); err == nil {
		c.Login = login
	}
	return c, nil
}

// PollDeviceFlow blocks until authorized or timeout. slowDown increases interval on 429.
func PollDeviceFlow(clientID, deviceCode string, intervalSec int) (Credential, error) {
	if intervalSec <= 0 {
		intervalSec = 5
	}
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(intervalSec) * time.Second)
		body := fmt.Sprintf("client_id=%s&device_code=%s&grant_type=urn:ietf:params:oauth:grant-type:device_code",
			clientID, deviceCode)
		req, err := http.NewRequest(http.MethodPost, accessTokenURL, strings.NewReader(body))
		if err != nil {
			return Credential{}, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return Credential{}, err
		}
		var raw map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()
		if errMsg, _ := raw["error"].(string); errMsg != "" {
			switch errMsg {
			case "authorization_pending":
				continue
			case "slow_down":
				intervalSec += 5
				continue
			default:
				return Credential{}, fmt.Errorf("github: %s", errMsg)
			}
		}
		tok, _ := raw["access_token"].(string)
		if tok == "" {
			continue
		}
		scopes, _ := raw["scope"].(string)
		c := Credential{Token: tok, Scopes: scopes, CreatedAt: time.Now().UTC()}
		login, err := fetchLogin(tok)
		if err == nil {
			c.Login = login
		}
		return c, nil
	}
	return Credential{}, fmt.Errorf("github device flow timed out")
}

func fetchLogin(token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, apiUserURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var u struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", err
	}
	if u.Login == "" {
		return "", fmt.Errorf("empty login")
	}
	return u.Login, nil
}

// ListRepos returns up to 100 repos for the authenticated user.
func ListRepos(token string) ([]Repo, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user/repos?per_page=100&sort=updated", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var repos []Repo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}
	return repos, nil
}

// Repo is a minimal GitHub repository descriptor.
type Repo struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	Default  string `json:"default_branch"`
}

// SaveToken stores a pasted token and resolves login.
func SaveToken(wsRoot, token string) (Credential, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Credential{}, fmt.Errorf("empty token")
	}
	c := Credential{Token: token, CreatedAt: time.Now().UTC()}
	if login, err := fetchLogin(token); err == nil {
		c.Login = login
	}
	if err := Save(wsRoot, c); err != nil {
		return Credential{}, err
	}
	return c, nil
}

// AuthHeader returns Authorization header value for git HTTP.
func AuthHeader(token string) string {
	return "Bearer " + token
}

// ConfigureGitEnv sets GIT_CONFIG_PARAMETERS for token auth in subprocesses.
func ConfigureGitEnv(token string) []string {
	if token == "" {
		return nil
	}
	// Use http.extraHeader for fetch in worktrees.
	return []string{"GIT_HTTP_EXTRAHEADER=Authorization: " + AuthHeader(token)}
}

// JSON for API responses.
func (d DeviceStart) MarshalPublic() []byte {
	b, _ := json.Marshal(map[string]string{
		"user_code":        d.UserCode,
		"verification_uri": d.VerificationURI,
		"device_code":      d.DeviceCode,
	})
	return b
}

func EncodeCredential(c Credential) []byte {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(map[string]any{
		"login": c.Login,
		"scopes": c.Scopes,
		"created_at": c.CreatedAt,
	})
	return buf.Bytes()
}
