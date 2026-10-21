// Package cfqueue pulls messages from a Cloudflare Queue (HTTP pull consumer).
package cfqueue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const apiBase = "https://api.cloudflare.com/client/v4"

// PullBatch receives up to batchSize messages from a queue.
func PullBatch(accountID, queueID, token string, batchSize int) ([]Message, error) {
	if batchSize <= 0 {
		batchSize = 10
	}
	url := fmt.Sprintf("%s/accounts/%s/queues/%s/messages/pull", apiBase, accountID, queueID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Set("visibility_timeout_ms", "30000")
	q.Set("batch_size", fmt.Sprintf("%d", batchSize))
	req.URL.RawQuery = q.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("queue pull %d: %s", resp.StatusCode, string(body))
	}
	var wrap struct {
		Success bool `json:"success"`
		Result  struct {
			Messages []Message `json:"messages"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, err
	}
	return wrap.Result.Messages, nil
}

// AckBatch deletes handled messages.
func AckBatch(accountID, queueID, token string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/accounts/%s/queues/%s/messages/ack", apiBase, accountID, queueID)
	payload := map[string]any{"acks": ids}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		out, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("queue ack %d: %s", resp.StatusCode, string(out))
	}
	return nil
}

// Message is one queue delivery.
type Message struct {
	ID       string          `json:"id"`
	Body     json.RawMessage `json:"body"`
	Attempts int             `json:"attempts"`
}

// BuildEvent is cf.workersBuilds.worker.build.* payload (subset).
type BuildEvent struct {
	Type       string `json:"type"`
	WorkerName string `json:"workerName"`
	Branch     string `json:"branch"`
	CommitHash string `json:"commitHash"`
	RepoName   string `json:"repoName"`
	Message    string `json:"commitMessage"`
	Error      string `json:"error"`
}

// ParseBuildEvent decodes a queue body into a build event when possible.
func ParseBuildEvent(body json.RawMessage) (BuildEvent, bool) {
	var outer struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &outer); err != nil {
		return BuildEvent{}, false
	}
	var ev BuildEvent
	if err := json.Unmarshal(outer.Data, &ev); err != nil {
		_ = json.Unmarshal(body, &ev)
	}
	ev.Type = outer.Type
	if ev.WorkerName == "" && ev.CommitHash == "" {
		return BuildEvent{}, false
	}
	return ev, true
}
