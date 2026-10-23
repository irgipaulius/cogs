package cfprovision

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/cfclient"
	"github.com/irgipaulius/cogs/pkg/cftunnel"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/state"
)

// EnsureD1R2 creates optional D1 and R2 resources.
func EnsureD1R2(ctx context.Context, cfg Config, st *state.Store, app state.AppView, appDir string) error {
	m, err := launch.Load(appDir)
	if err != nil {
		return err
	}
	if m.IsCombined() {
		return nil
	}
	if strings.TrimSpace(m.D1Name) != "" {
		if err := wranglerTryCreate(st.WSRoot(), "d1", "create", m.D1Name); err != nil {
			return fmt.Errorf("d1: %w", err)
		}
		_ = st.RecordOwned(app.ID, "d1", m.D1Name, `{}`)
		logf(cfg.Log, "D1 %s", m.D1Name)
	}
	if m.Attachments && strings.TrimSpace(m.R2Bucket) != "" {
		if err := wranglerTryCreate(st.WSRoot(), "r2", "bucket", "create", m.R2Bucket); err != nil {
			return fmt.Errorf("r2: %w", err)
		}
		_ = st.RecordOwned(app.ID, "r2", m.R2Bucket, `{}`)
		logf(cfg.Log, "R2 %s", m.R2Bucket)
	}
	return nil
}

// EnsureTunnel creates tunnel and records tunnel id.
func EnsureTunnel(ctx context.Context, cfg Config, st *state.Store, app state.AppView, appDir string) (tunnelID string, err error) {
	if cfg.Client == nil {
		cfg.Client = &cfclient.Client{WSRoot: st.WSRoot()}
	}
	m, err := launch.Load(appDir)
	if err != nil {
		return "", err
	}
	accountID, err := cfg.Client.AccountID(ctx)
	if err != nil {
		return "", err
	}
	tunnelName := strings.TrimSpace(m.TunnelName)
	if tunnelName == "" {
		tunnelName = "cogs-" + app.ID
	}
	if err := wranglerTryCreate(st.WSRoot(), "tunnel", "create", tunnelName); err != nil {
		return "", fmt.Errorf("tunnel create: %w", err)
	}
	tunnelID, err = cftunnel.FindID(ctx, cfg.Client, accountID, tunnelName)
	if err != nil {
		return "", err
	}
	meta, _ := json.Marshal(map[string]string{"name": tunnelName})
	_ = st.RecordOwned(app.ID, "tunnel", tunnelID, string(meta))
	logf(cfg.Log, "tunnel %s (%s)", tunnelName, tunnelID)
	return tunnelID, nil
}

// EnsureWorker deploys Worker + custom domain routes for page host.
func EnsureWorker(ctx context.Context, cfg Config, st *state.Store, app state.AppView, appDir string) error {
	m, err := launch.Load(appDir)
	if err != nil || m.IsCombined() {
		return err
	}
	appCfg, err := appcfg.Load(appDir)
	if err != nil {
		return err
	}
	if appCfg.DeployMode == appcfg.DeployPull {
		return nil
	}
	pageHost, _ := Hosts(app, m)
	if pageHost == "" {
		return nil
	}
	worker, err := DeployWorker(st.WSRoot(), appDir, pageHost, m)
	if err != nil {
		return err
	}
	wmeta, _ := json.Marshal(map[string]string{"hostname": pageHost})
	_ = st.RecordOwned(app.ID, "worker", worker, string(wmeta))
	_ = st.ClaimHostname(app.ID, strings.ToLower(pageHost), "page")
	logf(cfg.Log, "worker %s", worker)
	return nil
}

// EnsureDNS configures tunnel ingress and DNS for API (and combined page).
func EnsureDNS(ctx context.Context, cfg Config, st *state.Store, app state.AppView, appDir, tunnelID string) error {
	if cfg.Client == nil {
		cfg.Client = &cfclient.Client{WSRoot: st.WSRoot()}
	}
	m, err := launch.Load(appDir)
	if err != nil {
		return err
	}
	pageHost, apiHost := Hosts(app, m)
	if pageHost == "" && apiHost == "" {
		return fmt.Errorf("no hostnames to configure")
	}
	accountID, err := cfg.Client.AccountID(ctx)
	if err != nil {
		return err
	}
	if tunnelID == "" {
		tunnelID, err = EnsureTunnel(ctx, cfg, st, app, appDir)
		if err != nil {
			return err
		}
	}
	localSvc := "http://" + listenAddr(m.BindHost, m.Port)
	if m.IsCombined() && pageHost != "" {
		if err := ensureTunnelIngress(ctx, cfg.Client, accountID, tunnelID, pageHost, localSvc); err != nil {
			return err
		}
		return ensureTunnelDNS(ctx, cfg, st, app.ID, accountID, tunnelID, pageHost, "page")
	}
	if apiHost != "" {
		if err := ensureTunnelIngress(ctx, cfg.Client, accountID, tunnelID, apiHost, localSvc); err != nil {
			return err
		}
		if err := ensureTunnelDNS(ctx, cfg, st, app.ID, accountID, tunnelID, apiHost, "api"); err != nil {
			return err
		}
	}
	return nil
}
