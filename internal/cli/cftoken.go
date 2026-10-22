package cli

import (
	"fmt"
	"net/http"
	"os/exec"
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
