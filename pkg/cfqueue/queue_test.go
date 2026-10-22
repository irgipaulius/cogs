package cfqueue

import (
	"encoding/base64"
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

func TestAckPayloadUsesLeaseObjects(t *testing.T) {
	got := string(AckPayload([]string{"l1", "", "l2"}))
	want := `{"acks":[{"lease_id":"l1"},{"lease_id":"l2"}],"retries":[]}`
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestMessageDecodesLeaseID(t *testing.T) {
	var m Message
	if err := json.Unmarshal([]byte(`{"id":"i","lease_id":"L","body":"x"}`), &m); err != nil || m.LeaseID != "L" {
		t.Fatalf("lease=%q err=%v", m.LeaseID, err)
	}
}

func TestParseBuildEventBase64Body(t *testing.T) {
	inner := `{"type":"cf.workersBuilds.worker.build.succeeded","source":{"type":"workersBuilds.worker","workerName":"cogs-goldilocks"},"payload":{"buildTriggerMetadata":{"branch":"main","commitHash":"c1"}}}`
	enc, _ := json.Marshal(base64.StdEncoding.EncodeToString([]byte(inner)))
	ev, ok := ParseBuildEvent(json.RawMessage(enc))
	if !ok || ev.WorkerName != "cogs-goldilocks" || ev.CommitHash != "c1" || !strings.Contains(ev.Type, "build.succeeded") {
		t.Fatalf("ok=%v ev=%+v", ok, ev)
	}
}
