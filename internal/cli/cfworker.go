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
	toml := fmt.Sprintf(`name = %q
compatibility_date = "2024-06-01"
workers_dev = false

[assets]
directory = %q
`, worker, assetsDir)
	tomlPath := filepath.Join(genDir, "wrangler.toml")
	if err := os.WriteFile(tomlPath, []byte(toml), 0o644); err != nil {
		return err
	}
	fmt.Println("→ Worker static assets deploy")
	if err := wranglerInDir(genDir, "deploy"); err != nil {
		return err
	}
	if pageHost != "" {
		if err := ensureWorkerCustomDomain(projectDir, worker, pageHost); err != nil {
			return err
		}
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

func ensureWorkerCustomDomain(projectDir, workerName, pageHost string) error {
	dnsToken, err := cfTokenDNS(projectDir, zoneNameFromHost(pageHost))
	if err != nil {
		return err
	}
	tunnelToken, err := cfTokenTunnel()
	if err != nil {
		return err
	}
	accountID, err := cloudflareAccountID()
	if err != nil {
		return err
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/domains", accountID)
	_, err = cfRequest(tunnelToken, "POST", apiURL, map[string]string{
		"hostname": pageHost,
		"service":  workerName,
		"zone_id":  "",
	})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
		fmt.Printf("  worker domain API: %v (continuing with DNS)\n", err)
	}
	parts := strings.SplitN(pageHost, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("bad page host %q", pageHost)
	}
	zoneID, err := findZoneID(dnsToken, parts[1])
	if err != nil {
		return err
	}
	target := workerName + "." + parts[1] // fallback; often workers.dev subdomain
	_ = target
	target = workerName + ".workers.dev"
	return ensureCNAMERecordCoGS(projectDir, workerName, "worker", pageHost, target, dnsToken, zoneID)
}
