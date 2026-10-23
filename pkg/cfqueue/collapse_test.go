package cfqueue

import "testing"

func TestCollapseBuildSuccessNineToOne(t *testing.T) {
	prefer := func(a, b string) bool {
		// commits c1..c9 — higher suffix wins
		return a > b
	}
	var deliveries []BuildDelivery
	for i, hash := range []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9"} {
		deliveries = append(deliveries, BuildDelivery{
			Index:   i,
			LeaseID: "L" + hash,
			Event: BuildEvent{
				Type:       "cf.workersBuilds.worker.build.succeeded",
				WorkerName: "cogs-goldilocks",
				CommitHash: hash,
			},
		})
	}
	appFor := func(ev BuildEvent) (string, bool) {
		if ev.WorkerName == "cogs-goldilocks" {
			return "goldilocks", true
		}
		return "", false
	}
	got := CollapseBuildSuccess(deliveries, appFor, prefer)
	if len(got) != 1 {
		t.Fatalf("apps=%d want 1", len(got))
	}
	w := got["goldilocks"]
	if w.Event.CommitHash != "c9" {
		t.Fatalf("commit=%q want c9", w.Event.CommitHash)
	}
}
