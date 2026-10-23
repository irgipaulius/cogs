package cfprovision

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cfclient"
	"github.com/irgipaulius/cogs/pkg/state"
)

// Teardown deletes Cloudflare resources recorded in app_owned.
func Teardown(ctx context.Context, c *cfclient.Client, st *state.Store, appID string) error {
	if c == nil {
		c = &cfclient.Client{WSRoot: st.WSRoot()}
	}
	owned, err := st.ListOwned(appID)
	if err != nil {
		return err
	}
	accountID, accErr := c.AccountID(ctx)
	for _, r := range owned {
		switch r.Kind {
		case "dns_record":
			var meta struct {
				ZoneID string `json:"zone_id"`
			}
			_ = json.Unmarshal([]byte(r.MetaJSON), &meta)
			if meta.ZoneID != "" && r.ExternalID != "" {
				if err := deleteDNSRecord(ctx, c, meta.ZoneID, r.ExternalID); err != nil {
					return fmt.Errorf("delete dns %s: %w", r.ExternalID, err)
				}
			}
		case "tunnel":
			if accErr == nil && r.ExternalID != "" {
				if err := deleteTunnel(ctx, c, accountID, r.ExternalID); err != nil {
					return fmt.Errorf("delete tunnel %s: %w", r.ExternalID, err)
				}
			}
		case "worker":
			name := strings.TrimSpace(r.ExternalID)
			if name != "" {
				if err := deleteWorker(st.WSRoot(), name); err != nil {
					return fmt.Errorf("delete worker %s: %w", name, err)
				}
			}
		case "d1":
			if name := strings.TrimSpace(r.ExternalID); name != "" {
				_ = wranglerTryCreate(st.WSRoot(), "d1", "delete", name)
			}
		case "r2":
			if name := strings.TrimSpace(r.ExternalID); name != "" {
				_ = wranglerTryCreate(st.WSRoot(), "r2", "bucket", "delete", name)
			}
		}
	}
	return st.ClearOwned(appID)
}
