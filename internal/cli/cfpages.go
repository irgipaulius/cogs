package cli

import (
	"fmt"
	"net/http"
	"strings"
)

func ensurePagesCustomDomain(projectDir, projectName, pageHost string) error {
	dnsToken, err := cfTokenDNS(projectDir)
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
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/pages/projects/%s/domains", accountID, projectName)
	_, err = cfRequest(tunnelToken, http.MethodPost, apiURL, map[string]string{"name": pageHost})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
		fmt.Printf("  pages domain API: %v (continuing with DNS CNAME)\n", err)
	}
	parts := strings.SplitN(pageHost, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("bad page host %q", pageHost)
	}
	zoneID, err := findZoneID(dnsToken, parts[1])
	if err != nil {
		return err
	}
	target := projectName + ".pages.dev"
	appName := projectName
	if err := ensureCNAMERecordCoGS(projectDir, appName, "pages", pageHost, target, dnsToken, zoneID); err != nil {
		return err
	}
	fmt.Printf("Pages UI: https://%s (CNAME → %s)\n", pageHost, target)
	return nil
}
