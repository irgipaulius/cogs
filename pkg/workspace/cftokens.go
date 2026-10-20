package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const cfCloudflareDir = ".cogs/cloudflare"

// CFTokensRegistry maps zones to token files under the workspace (shared by all apps).
type CFTokensRegistry struct {
	Entries []CFTokenEntry `json:"entries"`
}

// CFTokenEntry is one DNS API token scoped to one or more zones.
type CFTokenEntry struct {
	Zones []string `json:"zones"`
	File  string   `json:"file"`
}

func cloudflareDir(wsRoot string) string {
	return filepath.Join(wsRoot, cfCloudflareDir)
}

func registryPath(wsRoot string) string {
	return filepath.Join(cloudflareDir(wsRoot), "tokens.json")
}

func defaultTokenPath(wsRoot string) string {
	return filepath.Join(cloudflareDir(wsRoot), "dns.token")
}

func zoneTokenFile(zone string) string {
	zone = strings.TrimSpace(strings.ToLower(zone))
	zone = strings.ReplaceAll(zone, "..", ".")
	return "dns-" + zone + ".token"
}

// LoadCFTokensRegistry reads tokens.json (optional).
func LoadCFTokensRegistry(wsRoot string) (CFTokensRegistry, error) {
	var reg CFTokensRegistry
	b, err := os.ReadFile(registryPath(wsRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return reg, err
	}
	if err := json.Unmarshal(b, &reg); err != nil {
		return reg, err
	}
	return reg, nil
}

func readTokenFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", os.ErrNotExist
	}
	return t, nil
}

// DNSTokenForZone returns a workspace DNS token for zone (registry entry, then default dns.token).
func DNSTokenForZone(wsRoot, zone string) (string, error) {
	if wsRoot == "" {
		return "", os.ErrNotExist
	}
	zone = strings.ToLower(strings.TrimSpace(zone))
	if zone != "" {
		reg, _ := LoadCFTokensRegistry(wsRoot)
		dir := cloudflareDir(wsRoot)
		for _, e := range reg.Entries {
			for _, z := range e.Zones {
				if strings.EqualFold(strings.TrimSpace(z), zone) {
					if p, err := readTokenFile(filepath.Join(dir, e.File)); err == nil {
						return p, nil
					}
				}
			}
		}
	}
	return readTokenFile(defaultTokenPath(wsRoot))
}

// SaveDNSToken stores a token at workspace root for reuse across apps (zone-scoped file + registry).
func SaveDNSToken(wsRoot, zone, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return os.ErrInvalid
	}
	dir := cloudflareDir(wsRoot)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	zone = strings.ToLower(strings.TrimSpace(zone))
	fileName := zoneTokenFile(zone)
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(defaultTokenPath(wsRoot), []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	reg, _ := LoadCFTokensRegistry(wsRoot)
	found := false
	for i, e := range reg.Entries {
		for _, z := range e.Zones {
			if strings.EqualFold(z, zone) {
				reg.Entries[i].File = fileName
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		reg.Entries = append(reg.Entries, CFTokenEntry{
			Zones: []string{zone},
			File:  fileName,
		})
	}
	b, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(registryPath(wsRoot), append(b, '\n'), 0o600)
}
