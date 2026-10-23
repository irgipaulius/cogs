package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appbuild"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workerwrangler"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func deployWorkerStaticAssets(projectDir, pageHost string, m launch.Manifest) error {
	app, err := appcfg.Load(projectDir)
	if err != nil {
		return err
	}
	worker := strings.TrimSpace(app.WorkerName)
	if worker == "" {
		worker = m.PagesProject
	}
	if worker == "" {
		return fmt.Errorf("worker_name or pages_project required for static assets deploy")
	}
	if err := appbuild.Build(projectDir, nil, false); err != nil {
		return err
	}
	genDir, err := workerwrangler.WriteGenerated(projectDir, worker, pageHost, m.PagesDir)
	if err != nil {
		return err
	}
	if err := wranglerInDir(genDir, "deploy"); err != nil {
		return err
	}
	if pageHost != "" {
		fmt.Printf("Worker UI: https://%s (custom domain via wrangler routes)\n", pageHost)
	}
	return nil
}

func wranglerInDir(dir string, args ...string) error {
	wsRoot, _, _ := workspace.FindRoot(dir)
	cmd, err := wranglerCmd(wsRoot, args...)
	if err != nil {
		return err
	}
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
