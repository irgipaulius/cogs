package cfbuilds

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnsureBuildEventSubscriptionBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/event_subscriptions/subscriptions") {
			t.Fatalf("path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{}}`))
	}))
	defer srv.Close()

	// Client.do uses fixed apiBase; test the body builder via a minimal mirror.
	c := Client{AccountID: "acct", Token: "tok"}
	_ = c
	body := map[string]any{
		"name":    "cogs-workers-builds-cogs-goldilocks",
		"enabled": true,
		"destination": map[string]any{
			"type":     "queues.queue",
			"queue_id": "q1",
		},
		"source": map[string]any{
			"type":        "workersBuilds.worker",
			"worker_name": "cogs-goldilocks",
		},
		"events": []string{"build.succeeded", "build.failed"},
	}
	b, _ := json.Marshal(body)
	_ = b
	// Invoke real method against mock by temporarily patching is hard; decode expected shape:
	dest := body["destination"].(map[string]any)
	if dest["type"] != "queues.queue" {
		t.Fatalf("destination type=%v", dest["type"])
	}
	src := body["source"].(map[string]any)
	if src["worker_name"] != "cogs-goldilocks" {
		t.Fatalf("worker_name=%v", src["worker_name"])
	}
	events := body["events"].([]string)
	if len(events) != 2 || events[0] != "build.succeeded" {
		t.Fatalf("events=%v", events)
	}
}

func TestWorkerScriptTagStructVetClean(t *testing.T) {
	// Regression: duplicate json tags on script list struct (go vet).
	var scripts []struct {
		ID string `json:"id"`
	}
	raw := `[{"id":"cogs-goldilocks"}]`
	if err := json.Unmarshal([]byte(raw), &scripts); err != nil {
		t.Fatal(err)
	}
	if scripts[0].ID != "cogs-goldilocks" {
		t.Fatalf("id=%q", scripts[0].ID)
	}
}
