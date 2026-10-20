package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ensureDNSCapableToken re-checks DNS token (e.g. tunnel fix). Uses same flow as launch prerequisite.
func ensureDNSCapableToken(projectDir, zoneName string) error {
	return ensureLaunchDNSPrerequisite(projectDir, zoneName)
}

func tokenCanEditDNS(token, zoneName string) bool {
	zoneID, err := findZoneID(token, zoneName)
	if err != nil {
		return false
	}
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?per_page=1", zoneID)
	_, err = cfRequest(token, http.MethodGet, apiURL, nil)
	return err == nil
}

func writeEnvVar(projectDir, key, value string) error {
	envPath := filepath.Join(projectDir, ".env")
	var lines []string
	if b, err := os.ReadFile(envPath); err == nil {
		lines = strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	}
	found := false
	prefix := key + "="
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
		lines = append(lines, prefix+value)
	}
	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start()
}

func zoneNameFromHost(host string) string {
	parts := strings.SplitN(host, ".", 2)
	if len(parts) != 2 {
		return host
	}
	return parts[1]
}

func launchZoneName(pageHost, apiHost string) string {
	if pageHost != "" {
		if z := zoneNameFromHost(pageHost); z != pageHost {
			return z
		}
	}
	return zoneNameFromHost(apiHost)
}
