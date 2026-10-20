package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/runner"
)

type runnerLease struct {
	RunnerID  string `json:"runner_id"`
	ClaimedAt string `json:"claimed_at"`
}

func cmdLease(args []string) error {
	if len(args) < 1 || args[0] != "watch" {
		return fmt.Errorf("usage: cogs lease watch [--dir appdir]")
	}
	dir := "."
	for i := 1; i < len(args); i++ {
		if args[i] == "--dir" && i+1 < len(args) {
			dir = args[i+1]
			i++
		}
	}
	abs, err := resolveLaunchProjectDir(dir)
	if err != nil {
		return err
	}
	return runLeaseWatch(abs, 0)
}

func runLeaseWatch(projectDir string, interval time.Duration) error {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	localID, err := runner.EnsureID(projectDir)
	if err != nil {
		return err
	}
	envPath := filepath.Join(projectDir, ".env")
	cfg, err := config.Load(envPath)
	if err != nil {
		return err
	}
	apiURL := strings.TrimRight(cfg.PublicAPIURL, "/")
	if apiURL == "" {
		return fmt.Errorf("COGS_PUBLIC_API_URL required for lease watch")
	}
	leaseURL := apiURL + "/.well-known/cogs-runner"
	m, err := launch.Load(projectDir)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	ux.Info("lease watch " + leaseURL + " (local " + localID + ")")
	for {
		if globals.DryRun {
			return nil
		}
		remote, err := fetchRunnerLease(client, leaseURL)
		if err != nil {
			time.Sleep(interval)
			continue
		}
		if remote.RunnerID == "" {
			time.Sleep(interval)
			continue
		}
		if remote.RunnerID == localID {
			time.Sleep(interval)
			continue
		}
		if !leaseIsNewer(remote.ClaimedAt, cfg.RunnerClaimedAt) {
			time.Sleep(interval)
			continue
		}
		ux.Warn(fmt.Sprintf("yielding to runner %s (claimed %s)", remote.RunnerID, remote.ClaimedAt))
		_ = stopTunnel(projectDir, m.TunnelName)
		stopApp(projectDir)
		return nil
	}
}

func fetchRunnerLease(client *http.Client, url string) (runnerLease, error) {
	resp, err := client.Get(url)
	if err != nil {
		return runnerLease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return runnerLease{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	var l runnerLease
	if err := json.NewDecoder(resp.Body).Decode(&l); err != nil {
		return runnerLease{}, err
	}
	return l, nil
}

func leaseIsNewer(remoteClaim, localClaim string) bool {
	if remoteClaim == "" {
		return false
	}
	if localClaim == "" {
		return true
	}
	rt, err1 := time.Parse(time.RFC3339, remoteClaim)
	lt, err2 := time.Parse(time.RFC3339, localClaim)
	if err1 != nil || err2 != nil {
		return remoteClaim > localClaim
	}
	return rt.After(lt)
}

func startLeaseWatchBackground(projectDir string) error {
	if off := os.Getenv("COGS_LEASE_WATCH_OFF"); off == "1" || strings.EqualFold(off, "true") {
		return nil
	}
	envPath := filepath.Join(projectDir, ".env")
	cfg, err := config.Load(envPath)
	if err == nil && !cfg.LeaseWatch {
		return nil
	}
	if pid, ok := leaseWatchRunning(projectDir); ok {
		ux.Info(fmt.Sprintf("lease watch already running pid %d", pid))
		return nil
	}
	return spawnLeaseWatch(projectDir)
}
