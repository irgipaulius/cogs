package cfprovision

import (
	"fmt"
	"os"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appbuild"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workerwrangler"
	"github.com/irgipaulius/cogs/pkg/wranglerexec"
)

// DeployWorker builds frontend and deploys the generated Worker (custom domain routes).
func DeployWorker(wsRoot, appDir, pageHost string, m launch.Manifest) (workerName string, err error) {
	app, err := appcfg.Load(appDir)
	if err != nil {
		return "", err
	}
	worker := strings.TrimSpace(app.WorkerName)
	if worker == "" {
		worker = m.PagesProject
	}
	if worker == "" {
		return "", fmt.Errorf("worker_name or pages_project required")
	}
	if err := appbuild.Build(appDir, nil, false); err != nil {
		return "", err
	}
	genDir, err := workerwrangler.WriteGenerated(appDir, worker, pageHost, m.PagesDir)
	if err != nil {
		return "", err
	}
	cmd, err := wranglerexec.Command(wsRoot, "deploy")
	if err != nil {
		return "", err
	}
	cmd.Dir = genDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("worker deploy: %w", err)
	}
	return worker, nil
}

func deleteWorker(wsRoot, workerName string) error {
	return wranglerTryCreate(wsRoot, "delete", workerName)
}
