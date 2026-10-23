// Package cfqueue pulls messages from a Cloudflare Queue (HTTP pull consumer).
package cfqueue

import (
	"bytes"
	"encoding/base64"
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
	pb, _ := json.Marshal(map[string]int{"visibility_timeout_ms": 30000, "batch_size": batchSize})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(pb))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, FormatPullError(resp.StatusCode, string(body))
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

// leaseRef is one entry of the acks/retries arrays of the pull-consumer ack API.
type leaseRef struct {
	LeaseID string `json:"lease_id"`
}

// AckPayload builds the /messages/ack request body: {"acks":[{"lease_id":...}],"retries":[]}.
func AckPayload(leaseIDs []string) []byte {
	acks := make([]leaseRef, 0, len(leaseIDs))
	for _, id := range leaseIDs {
		if id != "" {
			acks = append(acks, leaseRef{LeaseID: id})
		}
	}
	b, _ := json.Marshal(struct {
		Acks    []leaseRef `json:"acks"`
		Retries []leaseRef `json:"retries"`
	}{acks, []leaseRef{}})
	return b
}

// AckBatch acknowledges handled messages by their lease IDs.
func AckBatch(accountID, queueID, token string, leaseIDs []string) error {
	if len(leaseIDs) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/accounts/%s/queues/%s/messages/ack", apiBase, accountID, queueID)
	b := AckPayload(leaseIDs)
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
	LeaseID  string          `json:"lease_id"`
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
	body = unwrapBody(body)
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
				Branch        string `json:"branch"`
				CommitHash    string `json:"commitHash"`
				CommitMessage string `json:"commitMessage"`
				RepoName      string `json:"repoName"`
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

// unwrapBody handles bodies delivered as a JSON string: pull consumers receive
// json/bytes content base64-encoded, and text content as a plain string.
func unwrapBody(body json.RawMessage) json.RawMessage {
	var str string
	if err := json.Unmarshal(body, &str); err != nil {
		return body
	}
	str = strings.TrimSpace(str)
	if dec, err := base64.StdEncoding.DecodeString(str); err == nil && json.Valid(dec) {
		return dec
	}
	if json.Valid([]byte(str)) {
		return json.RawMessage(str)
	}
	return body
}
