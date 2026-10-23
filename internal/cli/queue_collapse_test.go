package cli

import (
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/cfqueue"
	"github.com/irgipaulius/cogs/pkg/state"
)

func TestPlanQueueBuildsCollapsesNineToOne(t *testing.T) {
	root := t.TempDir()
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var msgs []cfqueue.Message
	for _, hash := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		body := []byte(`{"type":"cf.workersBuilds.worker.build.succeeded","source":{"workerName":"cogs-goldilocks"},"payload":{"buildTriggerMetadata":{"commitHash":"` + hash + `","branch":"main"}}}`)
		msgs = append(msgs, cfqueue.Message{LeaseID: "L" + hash, Body: body})
	}
	match := func(ev cfqueue.BuildEvent) (string, string) {
		if ev.WorkerName == "cogs-goldilocks" {
			return "goldilocks", "main"
		}
		return "", ""
	}
	_, enqueue := planQueueBuilds(root, st, msgs, match)
	if len(enqueue) != 1 {
		t.Fatalf("enqueue apps=%d want 1", len(enqueue))
	}
	if enqueue["goldilocks"] != "i" {
		t.Fatalf("commit=%q want i", enqueue["goldilocks"])
	}
	_ = filepath.Join(root, ".cogs")
}
