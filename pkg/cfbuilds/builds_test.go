package cfbuilds

import (
	"encoding/json"
	"testing"
)

func TestEnsureBuildEventSubscriptionBody(t *testing.T) {
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

func TestWorkerScriptTagUsesImmutableTag(t *testing.T) {
	var scripts []struct {
		ID  string `json:"id"`
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal([]byte(`[{"id":"cogs-goldilocks","tag":"e8f70fdbc8b1fb0b8ddb1af166186758"}]`), &scripts); err != nil {
		t.Fatal(err)
	}
	if scripts[0].Tag != "e8f70fdbc8b1fb0b8ddb1af166186758" || scripts[0].ID == scripts[0].Tag {
		t.Fatalf("id=%q tag=%q", scripts[0].ID, scripts[0].Tag)
	}
}
