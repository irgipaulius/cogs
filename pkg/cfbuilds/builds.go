// Package cfbuilds wraps Cloudflare Workers Builds API (repo connections + triggers).
package cfbuilds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const apiBase = "https://api.cloudflare.com/client/v4"

// Client calls Workers Builds endpoints.
type Client struct {
	AccountID string
	Token     string
	HTTP      *http.Client
}

func (c *Client) do(method, path string, body any) ([]byte, error) {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	url := apiBase + path
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	var wrap struct {
		Success bool            `json:"success"`
		Errors  []map[string]any `json:"errors"`
		Result  json.RawMessage `json:"result"`
	}
	_ = json.Unmarshal(out, &wrap)
	if !wrap.Success {
		return nil, fmt.Errorf("cloudflare api %s %s: %s", method, path, string(out))
	}
	return wrap.Result, nil
}

// RepoConnection upserts a GitHub repo connection (requires Workers CI Write + GitHub App installed).
func (c *Client) RepoConnection(providerAccountName, repoName string, repoID int64) error {
	path := fmt.Sprintf("/accounts/%s/builds/repos/connections", c.AccountID)
	body := map[string]any{
		"provider_type":         "github",
		"provider_account_name": providerAccountName,
		"repo_name":             repoName,
		"repo_id":               repoID,
	}
	_, err := c.do(http.MethodPut, path, body)
	return err
}

// Trigger configures build on branch for a worker.
func (c *Client) Trigger(workerTag, branch, buildCmd, deployCmd, rootDir string) error {
	path := fmt.Sprintf("/accounts/%s/builds/workers/%s/triggers", c.AccountID, workerTag)
	body := map[string]any{
		"branch":         branch,
		"build_command":  buildCmd,
		"deploy_command": deployCmd,
	}
	if rootDir != "" {
		body["root_directory"] = rootDir
	}
	_, err := c.do(http.MethodPost, path, body)
	return err
}

// WorkerRepo returns linked repo from triggers lookup.
func (c *Client) WorkerRepo(workerTag string) (repoName string, err error) {
	path := fmt.Sprintf("/accounts/%s/builds/workers/%s/triggers", c.AccountID, workerTag)
	b, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	var triggers []struct {
		RepoConnection struct {
			RepoName string `json:"repo_name"`
		} `json:"repo_connection"`
	}
	if err := json.Unmarshal(b, &triggers); err != nil {
		return "", err
	}
	for _, t := range triggers {
		if t.RepoConnection.RepoName != "" {
			return t.RepoConnection.RepoName, nil
		}
	}
	return "", nil
}

// GitHubAppMissingHint is shown when the Cloudflare GitHub App is not installed.
const GitHubAppMissingHint = "Install the Cloudflare GitHub App: https://dash.cloudflare.com/?to=/:account/workers-and-pages/builds/connect"

// IsGitHubAppError heuristically detects missing GitHub app install.
func IsGitHubAppError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "github") && (strings.Contains(s, "install") || strings.Contains(s, "connection"))
}
