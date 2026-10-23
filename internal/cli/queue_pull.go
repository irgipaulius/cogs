package cli

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cfclient"
	"github.com/irgipaulius/cogs/pkg/cftoken"
	"github.com/irgipaulius/cogs/pkg/cfqueue"
	"github.com/irgipaulius/cogs/pkg/deploy"
	"github.com/irgipaulius/cogs/pkg/reconcile"
	"github.com/irgipaulius/cogs/pkg/state"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

const defaultQueueName = "cogs-deploys"

var (
	queuePullLastErr string
	queuePullLogMu   sync.Mutex
)

func pullDeployQueueOnce(root string, ws workspace.Root) {
	queueID := strings.TrimSpace(os.Getenv("COGS_DEPLOY_QUEUE_ID"))
	if queueID == "" {
		queueID = strings.TrimSpace(os.Getenv("COGS_CF_QUEUE_ID"))
	}
	if queueID == "" {
		return
	}
	cf := &cfclient.Client{WSRoot: root, OAuth: wranglerOAuthToken}
	ctx := context.Background()
	token, err := cf.Token(ctx, cftoken.ScopeQueue)
	if err != nil {
		logQueuePullErr("queue pull token: " + err.Error())
		return
	}
	accountID, err := cf.AccountID(ctx)
	if err != nil {
		logQueuePullErr("queue pull account: " + err.Error())
		return
	}
	msgs, err := cfqueue.PullBatch(accountID, queueID, token, 10)
	if err != nil {
		logQueuePullErr("queue pull: " + err.Error())
		return
	}
	clearQueuePullErr()
	var acks []string
	for _, msg := range msgs {
		ev, ok := cfqueue.ParseBuildEvent(msg.Body)
		if !ok {
			acks = append(acks, msg.LeaseID)
			continue
		}
		appID, branch := matchBuildEvent(root, ws, ev)
		if appID == "" {
			acks = append(acks, msg.LeaseID)
			continue
		}
		if branch != "" && ev.Branch != "" && !strings.EqualFold(branch, ev.Branch) {
			acks = append(acks, msg.LeaseID)
			continue
		}
		ctx := context.Background()
		switch {
		case strings.Contains(ev.Type, "build.succeeded"):
			if st, err := state.Open(root); err == nil {
				mgr := reconcile.NewManager(st)
				mgr.Enqueue(appID, reconcile.Options{Commit: ev.CommitHash})
				log.Printf("queue: %s reconcile queued after Workers build %s", appID, ev.CommitHash)
			} else if _, err := deploy.RecordBuildSuccess(ctx, root, appID, ev.CommitHash); err != nil {
				log.Printf("queue: %s redeploy after build failed: %v", appID, err)
			} else {
				log.Printf("queue: %s redeployed after Workers build %s", appID, ev.CommitHash)
			}
		case strings.Contains(ev.Type, "build.failed"):
			errMsg := ev.Error
			if errMsg == "" {
				errMsg = "Workers build failed"
			}
			_ = deploy.RecordBuildFailure(root, appID, ev.CommitHash, errMsg)
			log.Printf("queue: %s Workers build failed: %s", appID, errMsg)
		}
		acks = append(acks, msg.LeaseID)
	}
	if err := cfqueue.AckBatch(accountID, queueID, token, acks); err != nil {
		logQueuePullErr("queue ack: " + err.Error())
	}
}

func logQueuePullErr(msg string) {
	queuePullLogMu.Lock()
	defer queuePullLogMu.Unlock()
	if msg == queuePullLastErr {
		return
	}
	queuePullLastErr = msg
	log.Print(msg)
}

func clearQueuePullErr() {
	queuePullLogMu.Lock()
	queuePullLastErr = ""
	queuePullLogMu.Unlock()
}

func matchBuildEvent(root string, ws workspace.Root, ev cfqueue.BuildEvent) (appID, deployBranch string) {
	worker := strings.TrimSpace(ev.WorkerName)
	repoHint := strings.ToLower(strings.TrimSpace(ev.RepoName))
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
		if worker != "" && name != "" && name == worker {
			return a.Name, app.DeployBranch
		}
		if repoHint != "" && app.GitRepo != "" {
			gr := strings.ToLower(app.GitRepo)
			if strings.Contains(gr, repoHint) || strings.Contains(repoHint, gr) ||
				strings.HasSuffix(gr, "/"+repoHint) {
				return a.Name, app.DeployBranch
			}
		}
	}
	return "", ""
}
