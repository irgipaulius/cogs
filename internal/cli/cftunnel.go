package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/launch"
)

type tunnelListResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
}

type tunnelConfigGet struct {
	Success bool `json:"success"`
	Result  struct {
		Config struct {
			Ingress []ingressRule `json:"ingress"`
		} `json:"config"`
	} `json:"result"`
}

type ingressRule struct {
	Hostname string `json:"hostname,omitempty"`
	Service  string `json:"service"`
}

type tunnelConfigPut struct {
	Config struct {
		Ingress []ingressRule `json:"ingress"`
	} `json:"config"`
}

// ensureTunnelIngress sets hostname → local service on a remotely-managed tunnel (no dashboard).
func ensureTunnelIngress(projectDir, tunnelName, hostname, localService string) error {
	accountID, err := cloudflareAccountID()
	if err != nil {
		return err
	}
	token, err := cfTokenTunnel()
	if err != nil {
		return err
	}
	tunnelID, err := findTunnelID(accountID, token, tunnelName)
	if err != nil {
		return err
	}
	ingress, err := getTunnelIngress(accountID, token, tunnelID)
	if err != nil {
		ingress = nil
	}
	ingress = mergeIngress(ingress, hostname, localService)
	body, _ := json.Marshal(tunnelConfigPut{Config: struct {
		Ingress []ingressRule `json:"ingress"`
	}{Ingress: ingress}})
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations", accountID, tunnelID)
	req, err := http.NewRequest(http.MethodPut, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if !out.Success {
		msg := "tunnel configuration failed"
		if len(out.Errors) > 0 {
			msg = out.Errors[0].Message
		}
		return fmt.Errorf("%s", msg)
	}
	fmt.Printf("Tunnel ingress: %s → %s\n", hostname, localService)
	return nil
}

func cloudflareAccountID() (string, error) {
	whoami, err := wranglerCapture("whoami", "--json")
	if err != nil {
		return "", err
	}
	var w whoamiJSON
	if err := json.Unmarshal([]byte(whoami), &w); err != nil || len(w.Accounts) == 0 {
		return "", fmt.Errorf("account id from whoami")
	}
	return w.Accounts[0].ID, nil
}

func findTunnelID(accountID, token, name string) (string, error) {
	id, err := findTunnelIDAPI(accountID, token, name)
	if err == nil {
		return id, nil
	}
	if id, err2 := findTunnelIDWranglerCLI(name); err2 == nil {
		return id, nil
	}
	return "", err
}

func findTunnelIDAPI(accountID, token, name string) (string, error) {
	q := url.Values{}
	q.Set("name", name)
	q.Set("is_deleted", "false")
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel?%s", accountID, q.Encode())
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var tl tunnelListResponse
	if err := json.Unmarshal(raw, &tl); err != nil {
		return "", err
	}
	if !tl.Success {
		return "", fmt.Errorf("list tunnels API failed")
	}
	for _, t := range tl.Result {
		if t.Name == name {
			return t.ID, nil
		}
	}
	if len(tl.Result) == 1 {
		return tl.Result[0].ID, nil
	}
	return "", fmt.Errorf("tunnel %q not found via API", name)
}

func findTunnelIDWranglerCLI(name string) (string, error) {
	out, err := wranglerCaptureQuiet("tunnel", "list")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, name) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.Contains(fields[0], "-") && len(fields[0]) >= 36 {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("tunnel %q not in wrangler tunnel list", name)
}

func getTunnelIngress(accountID, token, tunnelID string) ([]ingressRule, error) {
	apiURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations", accountID, tunnelID)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var cg tunnelConfigGet
	if err := json.Unmarshal(raw, &cg); err != nil {
		return nil, err
	}
	if !cg.Success {
		return nil, fmt.Errorf("get tunnel config")
	}
	return cg.Result.Config.Ingress, nil
}

func mergeIngress(existing []ingressRule, hostname, service string) []ingressRule {
	var out []ingressRule
	var catch ingressRule
	for _, r := range existing {
		if r.Hostname == "" && r.Service != "" {
			catch = r
			continue
		}
		if r.Hostname == hostname {
			continue
		}
		out = append(out, r)
	}
	out = append(out, ingressRule{Hostname: hostname, Service: service})
	if catch.Service == "" {
		catch = ingressRule{Service: "http_status:404"}
	}
	out = append(out, catch)
	return out
}

func tunnelFix(projectDir string) error {
	if err := ensureWranglerLogin(); err != nil {
		return err
	}
	m, err := launch.Load(projectDir)
	if err != nil {
		return err
	}
	apiHost, err := apiHostFromProject(projectDir, m)
	if err != nil {
		return err
	}
	pageHost, _ := pageHostFromProject(projectDir)
	localSvc := "http://" + listenAddr(m.BindHost, m.Port)
	zone := launchZoneName(pageHost, apiHost)
	if zone == "" {
		zone = zoneNameFromHost(apiHost)
	}
	if err := ensureDNSCapableToken(projectDir, zone); err != nil {
		return err
	}
	if m.IsCombined() {
		if err := ensureTunnelIngress(projectDir, m.TunnelName, apiHost, localSvc); err != nil {
			return err
		}
		if err := ensureTunnelDNS(projectDir, m.TunnelName, apiHost); err != nil {
			return err
		}
		fmt.Println("Done. Test: curl -s https://" + apiHost + "/health")
		return nil
	}
	if err := ensureTunnelIngress(projectDir, m.TunnelName, apiHost, localSvc); err != nil {
		return err
	}
	if err := ensureTunnelDNS(projectDir, m.TunnelName, apiHost); err != nil {
		return err
	}
	if pageHost != "" {
		_ = ensurePagesCustomDomain(projectDir, m.PagesProject, pageHost)
	}
	fmt.Println("Done. Test: curl -s https://" + apiHost + "/health")
	return nil
}

func pageHostFromProject(projectDir string) (string, error) {
	envPath := filepath.Join(projectDir, ".env")
	b, err := os.ReadFile(envPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "COGS_PUBLIC_PAGE_ORIGIN=") {
			u := strings.TrimSpace(strings.TrimPrefix(line, "COGS_PUBLIC_PAGE_ORIGIN="))
			u = strings.Trim(u, `"'`)
			u = strings.TrimPrefix(u, "https://")
			u = strings.TrimPrefix(u, "http://")
			if u != "" {
				return u, nil
			}
		}
	}
	return "", fmt.Errorf("COGS_PUBLIC_PAGE_ORIGIN not in .env")
}

func apiHostFromProject(projectDir string, m launch.Manifest) (string, error) {
	envPath := filepath.Join(projectDir, ".env")
	if b, err := os.ReadFile(envPath); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "COGS_PUBLIC_API_URL=") {
				u := strings.TrimPrefix(line, "COGS_PUBLIC_API_URL=")
				u = strings.TrimSpace(strings.Trim(u, `"'`))
				if u != "" {
					p, err := url.Parse(u)
					if err == nil && p.Host != "" {
						return p.Host, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("set COGS_PUBLIC_API_URL in .env or re-run cogs launch")
}

func ensureTunnelDNS(projectDir, tunnelName, fqdn string) error {
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
	tunnelID, err := findTunnelID(accountID, tunnelToken, tunnelName)
	if err != nil {
		return err
	}
	parts := strings.SplitN(fqdn, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("bad hostname %q", fqdn)
	}
	zoneID, err := findZoneID(dnsToken, parts[1])
	if err != nil {
		return err
	}
	target := tunnelID + ".cfargotunnel.com"
	appName := filepath.Base(projectDir)
	if m, err := launch.Load(projectDir); err == nil && m.Name != "" {
		appName = m.Name
	}
	if err := ensureCNAMERecordCoGS(projectDir, appName, "tunnel", fqdn, target, dnsToken, zoneID); err != nil {
		printManualDNS(fqdn, target)
		return fmt.Errorf("%w — create a Cloudflare API token with Zone.DNS Edit for this zone, set COGS_CF_API_TOKEN in .env, or add the CNAME manually", err)
	}
	return nil
}

func printManualDNS(fqdn, tunnelTarget string) {
	zone := zoneNameFromHost(fqdn)
	fmt.Printf("\n--- Add this DNS record (Cloudflare → %s → DNS) ---\n", zone)
	fmt.Println("  Delete any A/AAAA record for the same name first if present.")
	fmt.Printf("  Type: CNAME\n  Name: %s\n  Target: %s\n  Proxy: Proxied (orange cloud)\n", strings.SplitN(fqdn, ".", 2)[0], tunnelTarget)
	fmt.Println("----------------------------------------------------------------")
}

func findZoneID(token, zoneName string) (string, error) {
	apiURL := "https://api.cloudflare.com/client/v4/zones?name=" + url.QueryEscape(zoneName)
	req, _ := http.NewRequest(http.MethodGet, apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var zr struct {
		Success bool `json:"success"`
		Result  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &zr); err != nil || len(zr.Result) == 0 {
		return "", fmt.Errorf("zone %q not found on account", zoneName)
	}
	return zr.Result[0].ID, nil
}

