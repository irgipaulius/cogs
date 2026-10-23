package cfprovision

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appbuild"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workerwrangler"
	"github.com/irgipaulius/cogs/pkg/wranglerexec"
)

const pagesDistRel = ".cogs/pages-dist"

// DeployWorker builds frontend and deploys the generated Worker (custom domain routes).
func DeployWorker(wsRoot, appDir, pageHost, apiHost, releaseCommit string, m launch.Manifest) (workerName string, err error) {
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
	releaseCommit = strings.TrimSpace(releaseCommit)
	marker := filepath.Join(appDir, ".cogs", "worker-deploy-sha")
	if releaseCommit != "" {
		if b, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(b)) == releaseCommit {
			return worker, nil
		}
	}
	_ = appbuild.RestoreTrackedUIArtifacts(appDir, m.PagesDir)
	if err := appbuild.WritePageConfig(appDir, pageHost, apiHost, m); err != nil {
		return "", err
	}
	if err := appbuild.Build(appDir, nil, true); err != nil {
		return "", err
	}
	genDir, err := workerwrangler.WriteGenerated(appDir, worker, pageHost, pagesDistRel)
	if err != nil {
		return "", err
	}
	cmd, err := wranglerexec.Command(wsRoot, "deploy")
	if err != nil {
		return "", err
	}
	cmd.Dir = genDir
	logPath := filepath.Join(wsRoot, ".cogs", "run", "wrangler.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		logFile.Close()
		return "", fmt.Errorf("worker deploy: %w", err)
	}
	_ = logFile.Close()
	wranglerexec.RotateWorkspaceLog(logPath)
	if releaseCommit != "" {
		_ = os.WriteFile(marker, []byte(releaseCommit+"\n"), 0o644)
	}
	return worker, nil
}

func deleteWorker(wsRoot, workerName string) error {
	return wranglerTryCreate(wsRoot, "delete", workerName)
}
