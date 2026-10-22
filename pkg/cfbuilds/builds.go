// Package cfbuilds wraps Cloudflare Workers Builds API (repo connections + triggers).
package cfbuilds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	reqURL := apiBase + path
	req, err := http.NewRequest(method, reqURL, r)
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

// RepoConnection upserts a GitHub repo connection. Returns connection uuid when present.
func (c *Client) RepoConnection(providerAccountName, repoName string, repoID int64) (string, error) {
	path := fmt.Sprintf("/accounts/%s/builds/repos/connections", c.AccountID)
	body := map[string]any{
		"provider_type":         "github",
		"provider_account_name": providerAccountName,
		"repo_name":             repoName,
		"repo_id":               repoID,
	}
	b, err := c.do(http.MethodPut, path, body)
	if err != nil {
		return "", err
	}
	var result struct {
		ID   string `json:"id"`
		UUID string `json:"uuid"`
	}
	_ = json.Unmarshal(b, &result)
	if result.UUID != "" {
		return result.UUID, nil
	}
	return result.ID, nil
}

// WorkerScriptTag resolves the Workers Builds script tag (script id) for API paths.
func (c *Client) WorkerScriptTag(workerName string) (string, error) {
	path := fmt.Sprintf("/accounts/%s/workers/scripts", c.AccountID)
	b, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return workerName, err
	}
	var scripts []struct {
		ID  string `json:"id"`
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal(b, &scripts); err != nil {
		return "", err
	}
	for _, s := range scripts {
		if s.ID != workerName && !strings.EqualFold(s.ID, workerName) {
			continue
		}
		if strings.TrimSpace(s.Tag) == "" {
			return "", fmt.Errorf("worker %q listed but tag missing in scripts API", workerName)
		}
		return s.Tag, nil
	}
	return "", fmt.Errorf("worker script %q not found — deploy the Worker once", workerName)
}

// Trigger configures build on branch for a worker script tag.
func (c *Client) Trigger(scriptTag, branch, buildCmd, deployCmd, rootDir, repoConnectionUUID string) error {
	path := fmt.Sprintf("/accounts/%s/builds/workers/%s/triggers", c.AccountID, url.PathEscape(scriptTag))
	body := map[string]any{
		"branch":         branch,
		"build_command":  buildCmd,
		"deploy_command": deployCmd,
	}
	if rootDir != "" {
		body["root_directory"] = rootDir
	}
	if repoConnectionUUID != "" {
		body["repo_connection_uuid"] = repoConnectionUUID
		body["repo_connection_id"] = repoConnectionUUID
	}
	_, err := c.do(http.MethodPost, path, body)
	return err
}

// EnsureDeployQueue returns queue id, creating by name when missing.
func (c *Client) EnsureDeployQueue(name string) (string, error) {
	listPath := fmt.Sprintf("/accounts/%s/queues", c.AccountID)
	b, err := c.do(http.MethodGet, listPath, nil)
	if err == nil {
		var queues []struct {
			QueueID string `json:"queue_id"`
			Name    string `json:"queue_name"`
		}
		_ = json.Unmarshal(b, &queues)
		for _, q := range queues {
			if q.Name == name && q.QueueID != "" {
				return q.QueueID, nil
			}
		}
	}
	createPath := fmt.Sprintf("/accounts/%s/queues", c.AccountID)
	b, err = c.do(http.MethodPost, createPath, map[string]string{"queue_name": name})
	if err != nil {
		return "", err
	}
	var created struct {
		QueueID string `json:"queue_id"`
	}
	_ = json.Unmarshal(b, &created)
	if created.QueueID == "" {
		return "", fmt.Errorf("queue create: empty queue_id in %s", string(b))
	}
	return created.QueueID, nil
}

// EnsureHTTPPullConsumer registers an HTTP pull consumer on the queue.
func (c *Client) EnsureHTTPPullConsumer(queueID string) error {
	path := fmt.Sprintf("/accounts/%s/queues/%s/consumers", c.AccountID, queueID)
	_, err := c.do(http.MethodPost, path, map[string]string{"type": "http_pull"})
	if err != nil && strings.Contains(err.Error(), "already") {
		return nil
	}
	return err
}

// EnsureBuildEventSubscription sends Workers Builds events for one worker to the queue.
func (c *Client) EnsureBuildEventSubscription(queueID, workerName string) error {
	path := fmt.Sprintf("/accounts/%s/event_subscriptions/subscriptions", c.AccountID)
	body := map[string]any{
		"name":    "cogs-workers-builds-" + workerName,
		"enabled": true,
		"destination": map[string]any{
			"type":     "queues.queue",
			"queue_id": queueID,
		},
		"source": map[string]any{
			"type":        "workersBuilds.worker",
			"worker_name": workerName,
		},
		"events": []string{
			"build.succeeded",
			"build.failed",
		},
	}
	_, err := c.do(http.MethodPost, path, body)
	if err != nil && (strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "duplicate")) {
		return nil
	}
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

const GitHubAppMissingHint = "Install the Cloudflare GitHub App: https://dash.cloudflare.com/?to=/:account/workers-and-pages/builds/connect"

func IsGitHubAppError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "github") && (strings.Contains(s, "install") || strings.Contains(s, "connection"))
}
