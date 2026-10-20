package cli

import (
	"fmt"
	"os"

	"github.com/irgipaulius/cogs/pkg/launch"
)

func needsPublicHostnames(skipPages, skipTunnel bool) bool {
	return !skipPages || !skipTunnel
}

// explainLaunchNetworking clarifies that the UI uses Pages, not the tunnel.
func explainLaunchNetworking(m launch.Manifest, pageHost, apiHost string) {
	fmt.Println("\n── How this app is exposed ──")
	fmt.Println("  The browser loads the UI from Cloudflare Pages (static files).")
	fmt.Println("  The UI calls your Go API over HTTPS on a separate hostname.")
	fmt.Println("  Only the API hostname uses the Cloudflare Tunnel to reach this machine.")
	fmt.Println()
	fmt.Println("  Browser ──► https://" + pageHost + "  →  Pages (" + m.PagesProject + ".pages.dev)")
	fmt.Println("  Browser ──► https://" + apiHost + "  →  Tunnel → 127.0.0.1:" + fmt.Sprint(m.Port))
	fmt.Println()
	fmt.Println("  Two proxied CNAME records on your zone are required (cogs creates them for you):")
	fmt.Printf("    %s  →  %s.pages.dev\n", pageHost, m.PagesProject)
	fmt.Printf("    %s  →  <tunnel-id>.cfargotunnel.com\n", apiHost)
}

func printDNSPrerequisite(zoneName string) {
	fmt.Println("\n── Prerequisite: DNS API token ──")
	fmt.Println("  Wrangler login cannot create or edit DNS records.")
	fmt.Println("  Before deploy, you need a Cloudflare API token with:")
	fmt.Println("    • Zone → DNS → Edit  (include zone: " + zoneName + ")")
	fmt.Println("  Optional but useful: Account → Cloudflare Tunnel → Edit")
	fmt.Println()
	fmt.Println("  Steps:")
	fmt.Println("    1. Browser opens the Cloudflare “Create API token” page.")
	fmt.Println("    2. Start from template “Edit zone DNS”, limit to this zone.")
	fmt.Println("    3. Create token, copy it, paste here (saved as COGS_CF_API_TOKEN in .env).")
	fmt.Println()
	fmt.Println("  Or set COGS_CF_API_TOKEN in .env before running cogs launch again.")
}

func ensureLaunchDNSPrerequisite(projectDir, zoneName string) error {
	token, err := cfTokenDNS(projectDir)
	if err == nil && tokenCanEditDNS(token, zoneName) {
		fmt.Println("\n✓ DNS API token OK (can edit zone " + zoneName + ")")
		return nil
	}
	printDNSPrerequisite(zoneName)
	if !stdinIsInteractive() {
		return fmt.Errorf("missing DNS-capable token: set COGS_CF_API_TOKEN in .env (Zone DNS Edit for %s), then re-run cogs launch", zoneName)
	}
	openBrowser("https://dash.cloudflare.com/profile/api-tokens/create?template=edit-zone-dns")
	fmt.Printf("Scope the token to zone: %s\n", zoneName)
	fmt.Print("\nPaste API token (required to continue): ")
	pasted, err := askString("", "")
	if err != nil || pasted == "" {
		return fmt.Errorf("DNS API token is required for public hostnames — launch stopped")
	}
	if !tokenCanEditDNS(pasted, zoneName) {
		return fmt.Errorf("token cannot edit DNS for zone %q — check permissions and zone scope", zoneName)
	}
	if err := writeEnvVar(projectDir, "COGS_CF_API_TOKEN", pasted); err != nil {
		return err
	}
	fmt.Println("Saved COGS_CF_API_TOKEN to .env")
	return nil
}

func stdinIsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
