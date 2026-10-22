// Package cfqueue pulls messages from a Cloudflare Queue (HTTP pull consumer).
package cfqueue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	var direct BuildEvent
	if err := json.Unmarshal(body, &direct); err == nil && direct.WorkerName != "" {
		return direct, true
	}

	var envelope struct {
		Type    string          `json:"type"`
		Source  json.RawMessage `json:"source"`
		Payload json.RawMessage `json:"payload"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return BuildEvent{}, false
	}
	ev := BuildEvent{Type: envelope.Type}
	if len(envelope.Source) > 0 {
		var src struct {
			WorkerName string `json:"workerName"`
		}
		_ = json.Unmarshal(envelope.Source, &src)
		ev.WorkerName = src.WorkerName
	}
	payload := envelope.Payload
	if len(payload) == 0 {
		payload = envelope.Data
	}
	if len(payload) > 0 {
		var p struct {
			BuildTriggerMetadata struct {
				Branch       string `json:"branch"`
				CommitHash   string `json:"commitHash"`
				CommitMessage string `json:"commitMessage"`
				RepoName     string `json:"repoName"`
			} `json:"buildTriggerMetadata"`
			Error string `json:"error"`
		}
		_ = json.Unmarshal(payload, &p)
		ev.Branch = p.BuildTriggerMetadata.Branch
		ev.CommitHash = p.BuildTriggerMetadata.CommitHash
		ev.Message = p.BuildTriggerMetadata.CommitMessage
		ev.RepoName = p.BuildTriggerMetadata.RepoName
		if ev.Error == "" {
			ev.Error = p.Error
		}
	}
	if ev.WorkerName == "" && !strings.Contains(ev.Type, "build.") {
		return BuildEvent{}, false
	}
	return ev, true
}
