package cfqueue

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseBuildEventEnvelope(t *testing.T) {
	body := json.RawMessage(`{
	  "type": "cf.workersBuilds.worker.build.succeeded",
	  "source": { "type": "workersBuilds.worker", "workerName": "cogs-goldilocks" },
	  "payload": {
	    "buildTriggerMetadata": {
	      "branch": "main",
	      "commitHash": "abc123",
	      "repoName": "irgipaulius/CoGS"
	    }
	  }
	}`)
	ev, ok := ParseBuildEvent(body)
	if !ok {
		t.Fatal("expected ok")
	}
	if ev.WorkerName != "cogs-goldilocks" {
		t.Fatalf("worker=%q", ev.WorkerName)
	}
	if ev.Branch != "main" || ev.CommitHash != "abc123" {
		t.Fatalf("branch=%q commit=%q", ev.Branch, ev.CommitHash)
	}
	if !strings.Contains(ev.Type, "build.succeeded") {
		t.Fatalf("type=%q", ev.Type)
	}
}
