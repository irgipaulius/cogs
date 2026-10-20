// Package dnsguard prevents CoGS from clobbering DNS records owned by other apps.
package dnsguard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileName = "dns-ownership.json"

// Record describes who manages a public hostname.
type Record struct {
	App    string `json:"app"`
	Target string `json:"target"`
	Kind   string `json:"kind"` // tunnel, pages, combined
}

type file struct {
	Records map[string]Record `json:"records"`
}

func path(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", fileName)
}

// Load reads ownership map.
func Load(wsRoot string) (map[string]Record, error) {
	p := path(wsRoot)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Record{}, nil
		}
		return nil, err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if f.Records == nil {
		f.Records = map[string]Record{}
	}
	return f.Records, nil
}

// Save writes ownership map.
func Save(wsRoot string, records map[string]Record) error {
	if err := os.MkdirAll(filepath.Join(wsRoot, ".cogs"), 0o750); err != nil {
		return err
	}
	f := file{Records: records}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path(wsRoot), b, 0o644)
}

// AssertCanManage returns an error if fqdn is owned by another CoGS app.
func AssertCanManage(wsRoot, fqdn, appName, newTarget, kind string) error {
	fqdn = strings.ToLower(strings.TrimSpace(fqdn))
	appName = strings.TrimSpace(appName)
	newTarget = strings.TrimSuffix(strings.TrimSpace(newTarget), ".")
	recs, err := Load(wsRoot)
	if err != nil {
		return err
	}
	existing, ok := recs[fqdn]
	if !ok {
		return nil
	}
	if existing.App == appName {
		return nil
	}
	if strings.EqualFold(strings.TrimSuffix(existing.Target, "."), newTarget) {
		return nil
	}
	return fmt.Errorf("DNS guard: %s is managed by CoGS app %q (%s → %s), not %q — pick another hostname or remove the other app first",
		fqdn, existing.App, existing.Kind, existing.Target, appName)
}

// Claim records successful DNS provisioning for an app.
func Claim(wsRoot, fqdn, appName, target, kind string) error {
	recs, err := Load(wsRoot)
	if err != nil {
		return err
	}
	fqdn = strings.ToLower(strings.TrimSpace(fqdn))
	recs[fqdn] = Record{
		App:    strings.TrimSpace(appName),
		Target: strings.TrimSuffix(strings.TrimSpace(target), "."),
		Kind:   kind,
	}
	return Save(wsRoot, recs)
}
