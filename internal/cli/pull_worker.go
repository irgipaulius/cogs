package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appbuild"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/workerwrangler"
)

func deployPullWorkerUI(projectDir, pageHost, apiHost string) error {
	pageHost = strings.TrimSpace(pageHost)
	apiHost = strings.TrimSpace(apiHost)
	if pageHost == "" || apiHost == "" {
		return fmt.Errorf("page and api hostnames required for Worker deploy")
	}
	app, err := appcfg.Load(projectDir)
	if err != nil {
		return err
	}
	worker := strings.TrimSpace(app.WorkerName)
	if worker == "" {
		worker = app.PagesProject
	}
	apiURL := "https://" + strings.TrimPrefix(apiHost, "https://")
	pageURL := "https://" + strings.TrimPrefix(pageHost, "https://")
	env := []string{
		"COGS_PUBLIC_API_URL=" + apiURL,
		"COGS_PUBLIC_PAGE_ORIGIN=" + pageURL,
	}
	if err := appbuild.Build(projectDir, env, false); err != nil {
		return err
	}
	genDir, err := workerwrangler.WriteGenerated(projectDir, worker, pageHost, app.PagesDir)
	if err != nil {
		return err
	}
	cmd, err := wranglerCmd("deploy")
	if err != nil {
		return err
	}
	cmd.Dir = genDir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wrangler deploy: %w", err)
	}
	fmt.Printf("Pull mode: Worker UI deployed at https://%s\n", pageHost)
	return nil
}

func pullBuildCommand(apiHost, pageHost string) string {
	apiURL := "https://" + strings.TrimPrefix(strings.TrimSpace(apiHost), "https://")
	pageURL := "https://" + strings.TrimPrefix(strings.TrimSpace(pageHost), "https://")
	// Build UI then deploy generated wrangler (routes + workers_dev=false).
	return fmt.Sprintf(
		`(test -f frontend/package.json && cd frontend && npm ci && COGS_PUBLIC_API_URL=%q COGS_PUBLIC_PAGE_ORIGIN=%q npm run build && cd ..) || COGS_PUBLIC_API_URL=%q COGS_PUBLIC_PAGE_ORIGIN=%q npm run build; npx wrangler deploy --config .cogs/generated/worker/wrangler.toml`,
		apiURL, pageURL, apiURL, pageURL,
	)
}
