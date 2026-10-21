package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cfqueue"
	"github.com/irgipaulius/cogs/pkg/deploy"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

const defaultQueueName = "cogs-deploys"

func pullDeployQueueOnce(root string, ws workspace.Root) {
	queueID := strings.TrimSpace(os.Getenv("COGS_DEPLOY_QUEUE_ID"))
	if queueID == "" {
		queueID = strings.TrimSpace(os.Getenv("COGS_CF_QUEUE_ID"))
	}
	if queueID == "" {
		return
	}
	token, err := cfTokenTunnel()
	if err != nil {
		return
	}
	accountID, err := cloudflareAccountID()
	if err != nil {
		return
	}
	msgs, err := cfqueue.PullBatch(accountID, queueID, token, 10)
	if err != nil {
		return
	}
	var acks []string
	for _, msg := range msgs {
		ev, ok := cfqueue.ParseBuildEvent(msg.Body)
		if !ok {
			acks = append(acks, msg.ID)
			continue
		}
		appID, branch := matchBuildEvent(root, ws, ev)
		if appID == "" {
			acks = append(acks, msg.ID)
			continue
		}
		if branch != "" && ev.Branch != "" && !strings.EqualFold(branch, ev.Branch) {
			acks = append(acks, msg.ID)
			continue
		}
		ctx := context.Background()
		switch {
		case strings.Contains(ev.Type, "build.succeeded"):
			_, _ = deploy.RecordBuildSuccess(ctx, root, appID, ev.CommitHash)
		case strings.Contains(ev.Type, "build.failed"):
			errMsg := ev.Error
			if errMsg == "" {
				errMsg = "Workers build failed"
			}
			_ = deploy.RecordBuildFailure(root, appID, ev.CommitHash, errMsg)
		}
		acks = append(acks, msg.ID)
	}
	_ = cfqueue.AckBatch(accountID, queueID, token, acks)
}

func matchBuildEvent(root string, ws workspace.Root, ev cfqueue.BuildEvent) (appID, deployBranch string) {
	worker := strings.TrimSpace(ev.WorkerName)
	for _, a := range ws.Apps {
		dir := filepath.Join(root, a.Path)
		app, err := appcfg.Load(dir)
		if err != nil || app.DeployMode != appcfg.DeployPull {
			continue
		}
		name := strings.TrimSpace(app.WorkerName)
		if name == "" {
			name = app.PagesProject
		}
		if name != "" && name == worker {
			return a.Name, app.DeployBranch
		}
	}
	return "", ""
}
