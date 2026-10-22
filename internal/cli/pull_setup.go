package cli

import (
	"fmt"
	"strings"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/boot"
	"github.com/irgipaulius/cogs/pkg/cfbuilds"
	"github.com/irgipaulius/cogs/pkg/githubauth"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func setupPullModeBuildsBestEffort(projectDir string, app appcfg.App, pageHost, apiHost string) {
	fmt.Println("→ Pull mode (Workers Builds for UI; CoGS deploys backend on build events)")
	if err := setupPullModeBuilds(projectDir, app, pageHost, apiHost); err != nil {
		ux.Warn("pull mode setup: " + err.Error())
	}
}

func setupPullModeBuilds(projectDir string, app appcfg.App, pageHost, apiHost string) error {
	if app.DeployMode != appcfg.DeployPull {
		return nil
	}
	token, err := cfTokenTunnel()
	if err != nil {
		return err
	}
	accountID, err := cloudflareAccountID()
	if err != nil {
		return err
	}
	wsRoot, _, _ := workspace.FindRoot(projectDir)
	ghTok := githubauth.Token(wsRoot)
	if ghTok == "" {
		return fmt.Errorf("GitHub not signed in — run: cogs github login (pull mode UI builds need repo access)")
	}
	repos, err := githubauth.ListRepos(ghTok)
	if err != nil {
		return err
	}
	repoFull := strings.TrimSpace(app.GitRepo)
	var repoID int64
	var owner string
	for _, r := range repos {
		if r.FullName == repoFull {
			repoID = r.ID
			parts := strings.SplitN(r.FullName, "/", 2)
			if len(parts) == 2 {
				owner = parts[0]
			}
			break
		}
	}
	if repoID == 0 {
		return fmt.Errorf("git_repo %q not found in your GitHub account — check cogs.app.toml", repoFull)
	}
	worker := app.WorkerName
	if worker == "" {
		worker = app.PagesProject
	}
	if pageHost == "" || apiHost == "" {
		return fmt.Errorf("pull mode needs page and api hostnames (zone/subdomains in cogs.app.toml)")
	}
	if err := deployPullWorkerUI(projectDir, pageHost, apiHost); err != nil {
		return fmt.Errorf("initial Worker deploy: %w", err)
	}

	client := cfbuilds.Client{AccountID: accountID, Token: token}
	connID, err := client.RepoConnection(owner, repoFull, repoID)
	if err != nil {
		if cfbuilds.IsGitHubAppError(err) {
			return fmt.Errorf("%s\n%s", err.Error(), cfbuilds.GitHubAppMissingHint)
		}
		return err
	}
	scriptTag, err := client.WorkerScriptTag(worker)
	if err != nil {
		return fmt.Errorf("worker %q: %w (deploy the Worker once or check worker_name)", worker, err)
	}
	rootDir := strings.TrimSpace(app.RootDirectory)
	buildCmd := pullBuildCommand(apiHost, pageHost)
	deployCmd := "npx wrangler deploy"
	if err := client.Trigger(scriptTag, app.DeployBranch, buildCmd, deployCmd, rootDir, connID); err != nil {
		if cfbuilds.IsGitHubAppError(err) {
			return fmt.Errorf("%s\n%s", err.Error(), cfbuilds.GitHubAppMissingHint)
		}
		return err
	}
	fmt.Printf("Pull mode: Workers Builds trigger on %s → %s (branch %s)\n", repoFull, worker, app.DeployBranch)

	queueID, err := client.EnsureDeployQueue(defaultQueueName)
	if err != nil {
		return fmt.Errorf("deploy queue: %w", err)
	}
	if err := client.EnsureHTTPPullConsumer(queueID); err != nil {
		return fmt.Errorf("queue HTTP pull consumer: %w", err)
	}
	if err := client.EnsureBuildEventSubscription(queueID, worker); err != nil {
		return fmt.Errorf("build event subscription: %w", err)
	}
	envPath := boot.UpEnvPath(wsRoot)
	if err := boot.WriteUpEnv(wsRoot, map[string]string{"COGS_DEPLOY_QUEUE_ID": queueID}); err != nil {
		return fmt.Errorf("write %s: %w", envPath, err)
	}
	fmt.Printf("Wrote %s — restart cogs up to poll build events\n", envPath)
	return nil
}
