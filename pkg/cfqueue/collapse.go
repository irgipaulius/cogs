package cfqueue

import (
	"strings"
)

// BuildDelivery ties a parsed build event to its queue lease.
type BuildDelivery struct {
	Event   BuildEvent
	LeaseID string
	Index   int // position in the pulled batch (later = newer when timestamps tie)
}

// CommitPrefer picks the commit to deploy when both are non-empty.
// prefer returns true when a should win over b.
type CommitPrefer func(a, b string) bool

// CollapseBuildSuccess keeps the newest build.succeeded per app from a batch.
func CollapseBuildSuccess(deliveries []BuildDelivery, appFor func(BuildEvent) (appID string, ok bool), prefer CommitPrefer) map[string]BuildDelivery {
	out := map[string]BuildDelivery{}
	for _, d := range deliveries {
		if !strings.Contains(d.Event.Type, "build.succeeded") {
			continue
		}
		appID, ok := appFor(d.Event)
		if !ok || appID == "" {
			continue
		}
		cur, have := out[appID]
		if !have || preferCommit(d.Event.CommitHash, cur.Event.CommitHash, d.Index, cur.Index, prefer) {
			out[appID] = d
		}
	}
	return out
}

func preferCommit(aHash, bHash string, aIdx, bIdx int, prefer CommitPrefer) bool {
	aHash = strings.TrimSpace(aHash)
	bHash = strings.TrimSpace(bHash)
	if aHash == "" {
		return false
	}
	if bHash == "" {
		return true
	}
	if prefer != nil {
		if prefer(aHash, bHash) {
			return true
		}
		if prefer(bHash, aHash) {
			return false
		}
	}
	return aIdx > bIdx
}
