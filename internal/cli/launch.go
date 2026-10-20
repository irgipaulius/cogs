package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/launch"
)

func cmdLaunch(args []string) error {
	flags, err := parseLaunchFlags(args)
	if err != nil {
		return err
	}
	return runLaunch(flags)
}

func runLaunch(flags launchFlags) error {
	abs, err := resolveLaunchProjectDir(flags.dir)
	if err != nil {
		return err
	}
	m, err := launch.Load(abs)
	if err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	resolveListen(abs, &m)
	_ = launch.Save(abs, m)

	if !flags.quick {
		if err := ensureWranglerLogin(); err != nil {
			return err
		}
	}

	envPath := filepath.Join(abs, ".env")
	var pageHost, apiHost, domain string
	if !flags.pickZone {
		pageHost, apiHost, _ = hostsFromEnv(envPath)
		if pageHost != "" {
			domain = zoneFromPageHost(pageHost)
			fmt.Println("\nReusing hostnames from .env:")
			fmt.Println("  UI :", pageHost)
			fmt.Println("  API:", apiHost)
		}
	}
	if pageHost == "" {
		var err error
		domain, err = pickCloudflareZone()
		if err != nil {
			return err
		}
		domain = strings.TrimSpace(domain)
		if domain == "" {
			return fmt.Errorf("zone is required — add a site at dash.cloudflare.com or use cogs launch --skip-pages --skip-tunnel for local-only")
		}
		pageHost = m.PageSubdomain + "." + domain
		apiHost = m.APISubdomain + "." + domain
		fmt.Println("\nPlanned hostnames:")
		fmt.Println("  UI :", pageHost)
		fmt.Println("  API:", apiHost)
	}

	if flags.quick {
		fmt.Println("\nQuick redeploy — UI + Pages + migrate + API (skipping D1/R2/tunnel/DNS provisioning)")
	} else {
		explainLaunchNetworking(m, pageHost, apiHost)
	}
	if !flags.quick {
		explainBlobStorage(abs)
	}

	if !flags.yes {
		ok, err := askYesNo("Continue?", true)
		if err != nil || !ok {
			return nil
		}
	}

	if globals.DryRun {
		ux.Plan("launch dry-run", []string{
			"listen: " + listenAddr(m.BindHost, m.Port),
			"UI: https://" + pageHost,
			"API: https://" + apiHost,
			"quick: " + fmt.Sprint(flags.quick),
		})
		return nil
	}

	if err := ensureEnv(envPath, m, pageHost, apiHost); err != nil {
		return err
	}
	if err := ensureBlobStorage(abs); err != nil {
		return err
	}

	if !flags.quick && needsPublicHostnames(flags.skipPages, flags.skipTunnel) {
		if err := ensureLaunchDNSPrerequisite(abs, domain); err != nil {
			return err
		}
	}

	if !flags.quick {
		fmt.Println("\n→ Cloudflare D1 (edge read replica)")
		_ = wranglerTryCreate("d1", "create", m.D1Name)

		fmt.Println("→ R2 bucket (optional — edge sync via cogs sync; attachments use local COGS_BLOB_DIR by default)")
		_ = wranglerTryCreate("r2", "bucket", "create", m.R2Bucket)

		if !flags.skipTunnel {
			fmt.Println("→ Tunnel (create + ingress + API DNS)")
			_ = wranglerTryCreate("tunnel", "create", m.TunnelName)
		}
	}

	if !flags.skipUIBuild {
		if err := buildLaunchFrontend(abs, flags.quick); err != nil {
			return err
		}
	}

	if !flags.skipPages {
		fmt.Println("→ Pages deploy")
		if !flags.quick {
			_ = wranglerTryCreate("pages", "project", "create", m.PagesProject, "--production-branch=main")
		}
		cfgJS := filepath.Join(abs, m.PagesDir, "config.js")
		if err := writePageConfig(cfgJS, pageHost, apiHost); err != nil {
			return err
		}
		pagesDir := filepath.Join(abs, m.PagesDir)
		if err := wrangler("pages", "deploy", pagesDir, "--project-name="+m.PagesProject); err != nil {
			return err
		}
	}

	if !flags.quick && needsPublicHostnames(flags.skipPages, flags.skipTunnel) {
		fmt.Println("→ Custom hostnames (tunnel ingress + DNS CNAMEs + Pages domain)")
		if err := configurePublicHosts(abs, m, pageHost, apiHost); err != nil {
			return fmt.Errorf("public hostnames: %w\nRe-run after fixing DNS, or: cogs tunnel fix --dir %s", err, abs)
		}
	}

	fmt.Println("\n→ SQLite migrate + Go server")
	if !flags.quick {
		if err := ensureGoModTidy(abs); err != nil {
			return err
		}
	}
	if err := runGo(abs, "run", "./cmd/app", "migrate", "-env", ".env"); err != nil {
		return err
	}

	if !flags.skipTunnel {
		if err := applyDeployTakeover(abs, m); err != nil {
			return err
		}
	}

	ux.Title("Running locally")
	ux.Info("API listen " + listenAddr(m.BindHost, m.Port))
	if flags.skipTunnel {
		fmt.Println("Tunnel skipped (--skip-tunnel); API is local only.")
	} else {
		fmt.Printf("Public API (when tunnel is up): https://%s\n", apiHost)
	}
	if !flags.skipPages {
		fmt.Printf("Public UI: https://%s\n", pageHost)
	}

	switch flags.serveMode {
	case serveSkip:
		ux.OK("skipped API serve (--no-serve)")
	case serveBackground:
		if err := startAppBackground(abs); err != nil {
			return err
		}
	}

	if !flags.skipTunnel && !flags.noTunnelBG {
		ux.Step("Starting Cloudflare tunnel in background (via npx wrangler)…")
		if err := startTunnelBackground(abs, m.TunnelName); err != nil {
			ux.Warn(fmt.Sprintf("tunnel: %v — retry: cogs tunnel start %s", err, m.TunnelName))
		}
	} else if !flags.skipTunnel {
		ux.Info(fmt.Sprintf("Start tunnel when ready: cogs tunnel start %s", m.TunnelName))
	}

	if flags.leaseWatch && !flags.skipTunnel {
		if err := startLeaseWatchBackground(abs); err != nil {
			ux.Warn("lease watch: " + err.Error())
		}
	}

	if flags.serveMode == serveForeground {
		ux.Info("Starting server (Ctrl+C stops app; tunnel keeps running unless you cogs tunnel stop)…")
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sig
			os.Exit(0)
		}()
		return runGo(abs, "run", "./cmd/app", "serve", "-env", ".env")
	}
	return nil
}

func ensureEnv(path string, m launch.Manifest, pageHost, apiHost string) error {
	if _, err := os.Stat(path); err == nil {
		ux.Info("Using existing .env")
		if err := setEnvKey(path, "COGS_LISTEN", listenAddr(m.BindHost, m.Port)); err != nil {
			return err
		}
		return appendEnvKeys(path, map[string]string{
			"COGS_PUBLIC_PAGE_ORIGIN": "https://" + pageHost,
			"COGS_PUBLIC_API_URL":     "https://" + apiHost,
			"COGS_OBS":                "file",
			"COGS_BLOB_STORE":         "fs",
			"COGS_BLOB_DIR":           "data/blobs",
		})
	}
	secret, _ := randomHex(32)
	content := fmt.Sprintf(`COGS_LISTEN=%s
COGS_DB_PATH=data/todo.db
COGS_SESSION_SECRET=%s
COGS_PUBLIC_PAGE_ORIGIN=https://%s
COGS_PUBLIC_API_URL=https://%s
COGS_BLOB_STORE=fs
COGS_BLOB_DIR=data/blobs
COGS_MAX_UPLOAD_BYTES=%d
# Optional R2 (only if COGS_BLOB_STORE=r2 or cogs sync to edge):
COGS_R2_BUCKET=%s
COGS_R2_ACCOUNT_ID=
COGS_R2_ACCESS_KEY_ID=
COGS_R2_SECRET_ACCESS_KEY=
COGS_D1_ACCOUNT_ID=
COGS_D1_DATABASE_ID=
COGS_D1_API_TOKEN=
# Required for automatic DNS on launch (Zone DNS Edit); filled when you paste token at prompt
COGS_CF_API_TOKEN=
COGS_OBS=file
COGS_OBS_LOG=
`, listenAddr(m.BindHost, m.Port), secret, pageHost, apiHost, m.MaxUploadBytes, m.R2Bucket)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	fmt.Println("Wrote", path)
	return nil
}

func setEnvKey(path, key, value string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	prefix := key + "="
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, prefix+value)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o600)
}

func appendEnvKeys(path string, kv map[string]string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := string(b)
	for k, v := range kv {
		if strings.Contains(lines, k+"=") {
			continue
		}
		lines += fmt.Sprintf("%s=%s\n", k, v)
	}
	return os.WriteFile(path, []byte(lines), 0o600)
}

func writePageConfig(path, pageHost, apiHost string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`// Generated by cogs launch at %s
window.COGS_CONFIG = {
  pageOrigin: "https://%s",
  apiBase: "https://%s",
  localKey: "cogs_todo_v1",
  offlineQueueKey: "cogs_todo_offline_v1",
};
`, time.Now().Format(time.RFC3339), pageHost, apiHost)
	return os.WriteFile(path, []byte(body), 0o644)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func runGo(dir string, args ...string) error {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
