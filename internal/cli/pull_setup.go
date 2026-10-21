package cli

import (
	"fmt"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cfbuilds"
	"github.com/irgipaulius/cogs/pkg/githubauth"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func setupPullModeBuilds(projectDir string, app appcfg.App, pageHost string) error {
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
	ghTok, err := requireGitHubToken(wsRoot)
	if err != nil {
		return err
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
	client := cfbuilds.Client{AccountID: accountID, Token: token}
	if err := client.RepoConnection(owner, repoFull, repoID); err != nil {
		if cfbuilds.IsGitHubAppError(err) {
			return fmt.Errorf("%s\n%s", err.Error(), cfbuilds.GitHubAppMissingHint)
		}
		return err
	}
	worker := app.WorkerName
	if worker == "" {
		worker = app.PagesProject
	}
	rootDir := strings.TrimSpace(app.RootDirectory)
	buildCmd := "npm run build"
	deployCmd := "npx wrangler deploy"
	if err := client.Trigger(worker, app.DeployBranch, buildCmd, deployCmd, rootDir); err != nil {
		if cfbuilds.IsGitHubAppError(err) {
			return fmt.Errorf("%s\n%s", err.Error(), cfbuilds.GitHubAppMissingHint)
		}
		return err
	}
	fmt.Printf("Pull mode: Workers Builds trigger on %s → %s (branch %s)\n", repoFull, worker, app.DeployBranch)
	fmt.Printf("Set COGS_DEPLOY_QUEUE_ID to your %q queue id after event subscription.\n", defaultQueueName)
	_ = pageHost
	return nil
}
