package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/launch"
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
	genDir := filepath.Join(projectDir, ".cogs", "generated", "worker")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return err
	}
	assetsDir, err := filepath.Abs(filepath.Join(projectDir, m.PagesDir))
	if err != nil {
		return err
	}
	routeBlock := ""
	if pageHost != "" {
		routeBlock = fmt.Sprintf(`
[[routes]]
pattern = %q
custom_domain = true
`, pageHost)
	}
	toml := fmt.Sprintf(`name = %q
compatibility_date = "2024-06-01"
workers_dev = false
%s
[assets]
directory = %q
`, worker, routeBlock, assetsDir)
	tomlPath := filepath.Join(genDir, "wrangler.toml")
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
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
	cmd, err := wranglerCmd(args...)
	if err != nil {
		return err
	}
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
