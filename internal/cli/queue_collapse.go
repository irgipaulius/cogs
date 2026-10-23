package cli

import (
	"context"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/cfqueue"
	"github.com/irgipaulius/cogs/pkg/gitrepo"
	"github.com/irgipaulius/cogs/pkg/state"
)

func planQueueBuilds(root string, st *state.Store, msgs []cfqueue.Message, match func(cfqueue.BuildEvent) (appID, branch string)) (acks []string, enqueue map[string]string) {
	acks = make([]string, 0, len(msgs))
	enqueue = map[string]string{}
	var deliveries []cfqueue.BuildDelivery
	for i, msg := range msgs {
		ev, ok := cfqueue.ParseBuildEvent(msg.Body)
		if !ok {
			if msg.LeaseID != "" {
				acks = append(acks, msg.LeaseID)
			}
			continue
		}
		if strings.Contains(ev.Type, "build.failed") {
			if msg.LeaseID != "" {
				acks = append(acks, msg.LeaseID)
			}
			continue
		}
		deliveries = append(deliveries, cfqueue.BuildDelivery{Event: ev, LeaseID: msg.LeaseID, Index: i})
	}
	prefer := func(a, b string) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if gitrepo.IsAncestor(ctx, root, a, b) {
			return false
		}
		if gitrepo.IsAncestor(ctx, root, b, a) {
			return true
		}
		return a > b
	}
	appFor := func(ev cfqueue.BuildEvent) (string, bool) {
		appID, branch := match(ev)
		if appID == "" {
			return "", false
		}
		if branch != "" && ev.Branch != "" && !strings.EqualFold(branch, ev.Branch) {
			return "", false
		}
		return appID, true
	}
	winners := cfqueue.CollapseBuildSuccess(deliveries, appFor, prefer)
	ctx := context.Background()
	for _, d := range deliveries {
		if d.LeaseID == "" {
			continue
		}
		appID, ok := appFor(d.Event)
		if !ok {
			acks = append(acks, d.LeaseID)
			continue
		}
		w, isWin := winners[appID]
		if !isWin || d.LeaseID != w.LeaseID {
			acks = append(acks, d.LeaseID)
			continue
		}
		commit := strings.TrimSpace(d.Event.CommitHash)
		if commit == "" {
			acks = append(acks, d.LeaseID)
			continue
		}
		current, _, _ := st.CurrentRelease(appID)
		if gitrepo.ShouldSkipDeployCommit(ctx, root, commit, current) {
			acks = append(acks, d.LeaseID)
			continue
		}
		enqueue[appID] = commit
		acks = append(acks, d.LeaseID)
	}
	return acks, enqueue
}
