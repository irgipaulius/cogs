package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cftoken"
)

type zoneEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type zonesResponse struct {
	Success bool        `json:"success"`
	Result  []zoneEntry `json:"result"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type whoamiJSON struct {
	Accounts []struct {
		ID string `json:"id"`
	} `json:"accounts"`
}

// onlyCloudflareZone returns the sole zone on the account. More than one zone needs cogs.app.toml.
func onlyCloudflareZone() (string, error) {
	zones, err := fetchCloudflareZones()
	if err != nil {
		return "", err
	}
	if len(zones) != 1 {
		return "", fmt.Errorf("found %d Cloudflare zones", len(zones))
	}
	return zones[0].Name, nil
}

// pickCloudflareZone lists zones on the account and lets the user choose.
func pickCloudflareZone() (string, error) {
	fmt.Print(`
A Cloudflare "zone" is a domain you added to Cloudflare — the apex name only.
  Example: if your site is www.example.com, the zone is example.com

CoGS will create:
  todo.<zone>      → Pages (the todo UI)
  todo-api.<zone>  → Tunnel (API to SQLite on this Mac)

`)
	zones, err := fetchCloudflareZones()
	if err != nil {
		fmt.Printf("Could not list zones automatically: %v\n", err)
		fmt.Println("Enter your zone name manually (you must own it on Cloudflare).")
		return askString("Zone (apex domain)", "")
	}
	if len(zones) == 0 {
		fmt.Println("No zones found on this Cloudflare account.")
		fmt.Println("Add a domain first: Cloudflare dashboard → Websites → Add a site")
		fmt.Println("Or use Cloudflare's free workers.dev-only flow with: cogs launch --skip-pages --skip-tunnel")
		return askString("Zone (apex domain)", "")
	}
	if len(zones) == 1 {
		fmt.Printf("Found 1 zone: %s\n", zones[0].Name)
		ok, err := askYesNo("Use this zone?", true)
		if err != nil {
			return "", err
		}
		if ok {
			return zones[0].Name, nil
		}
		return askString("Zone (apex domain)", zones[0].Name)
	}
	fmt.Println("Your Cloudflare zones:")
	for i, z := range zones {
		fmt.Printf("  %d) %s\n", i+1, z.Name)
	}
	for {
		choice, err := askString("Pick a zone by number or type a domain", "1")
		if err != nil {
			return "", err
		}
		var n int
		if _, err := fmt.Sscanf(choice, "%d", &n); err == nil {
			if n >= 1 && n <= len(zones) {
				return zones[n-1].Name, nil
			}
		}
		if strings.Contains(choice, ".") {
			return strings.TrimSpace(choice), nil
		}
		fmt.Println("Enter a number from the list or a full domain like example.com")
	}
}

func fetchCloudflareZones() ([]zoneEntry, error) {
	whoami, err := wranglerCapture("whoami", "--json")
	if err != nil {
		return nil, err
	}
	var w whoamiJSON
	if err := json.Unmarshal([]byte(whoami), &w); err != nil || len(w.Accounts) == 0 {
		return nil, fmt.Errorf("parse whoami")
	}
	accountID := w.Accounts[0].ID
	token, err := wranglerOAuthToken()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?account.id=%s&per_page=50", accountID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var zr zonesResponse
	if err := json.Unmarshal(body, &zr); err != nil {
		return nil, err
	}
	if !zr.Success {
		msg := "zones API failed"
		if len(zr.Errors) > 0 {
			msg = zr.Errors[0].Message
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return zr.Result, nil
}

func wranglerOAuthToken() (string, error) {
	return cftoken.WranglerOAuthToken()
}

func wranglerConfigPath() (string, error) {
	if h := os.Getenv("WRANGLER_HOME"); h != "" {
		return filepath.Join(h, "config", "default.toml"), nil
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Preferences", ".wrangler", "config", "default.toml"), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, ".wrangler", "config", "default.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", ".wrangler", "config", "default.toml"), nil
}
