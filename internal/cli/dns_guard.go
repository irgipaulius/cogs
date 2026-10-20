package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/dnsguard"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func workspaceRootFromProject(projectDir string) string {
	if root, _, err := workspace.FindRoot(projectDir); err == nil {
		return root
	}
	return filepath.Dir(filepath.Dir(projectDir))
}

func ensureCNAMERecordCoGS(projectDir, appName, kind, fqdn, target, token, zoneID string) error {
	ws := workspaceRootFromProject(projectDir)
	if err := dnsguard.AssertCanManage(ws, fqdn, appName, target, kind); err != nil {
		return err
	}
	target = strings.TrimSuffix(strings.TrimSpace(target), ".")
	records, err := listDNSRecords(token, zoneID, fqdn)
	if err != nil {
		return err
	}
	owned, _ := dnsguard.Load(ws)
	for _, rec := range records {
		if rec.Type != "CNAME" && rec.Type != "A" && rec.Type != "AAAA" {
			continue
		}
		content := strings.TrimSuffix(strings.TrimSpace(rec.Content), ".")
		if strings.EqualFold(content, target) {
			continue
		}
		if o, ok := owned[strings.ToLower(fqdn)]; ok && o.App == appName {
			continue
		}
		return fmt.Errorf("DNS guard: %s already has %s → %s (not this CoGS app); pick another hostname or remove the record in Cloudflare",
			fqdn, rec.Type, rec.Content)
	}
	if err := ensureCNAMERecord(token, zoneID, fqdn, target); err != nil {
		return err
	}
	return dnsguard.Claim(ws, fqdn, appName, target, kind)
}
