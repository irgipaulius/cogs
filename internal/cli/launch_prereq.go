package cli

import (
	"fmt"
	"os"

	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func needsPublicHostnames(skipPages, skipTunnel bool) bool {
	return !skipTunnel
}

func needsSplitAPIDNS(skipPages, skipTunnel bool) bool {
	return !skipTunnel
}

// explainLaunchNetworking clarifies exposure model from cogs.launch.json.
func explainLaunchNetworking(m launch.Manifest, pageHost, apiHost string) {
	local := listenAddr(m.BindHost, m.Port)
	fmt.Println("\n── How this app is exposed ──")
	if m.IsCombined() {
		fmt.Println("  Combined app: embedded UI + API on one public hostname (Cloudflare Tunnel).")
		fmt.Printf("  Browser ──► https://%s  →  Tunnel → %s\n", pageHost, local)
		fmt.Println()
		fmt.Println("  One proxied CNAME on your zone (cogs creates it):")
		fmt.Printf("    %s  →  <tunnel-id>.cfargotunnel.com\n", pageHost)
		return
	}
	fmt.Println("  The browser loads the UI from a Cloudflare Worker (static assets only).")
	fmt.Println("  The UI calls your Go API over HTTPS on a separate hostname.")
	fmt.Println("  Only the API hostname uses the Cloudflare Tunnel to reach this machine.")
	fmt.Println()
	fmt.Printf("  Browser ──► https://%s  →  Worker custom domain\n", pageHost)
	fmt.Printf("  Browser ──► https://%s  →  Tunnel → %s\n", apiHost, local)
	fmt.Println()
	fmt.Println("  wrangler deploy attaches the UI hostname; cogs creates API tunnel DNS:")
	fmt.Printf("    %s  →  <tunnel-id>.cfargotunnel.com\n", apiHost)
}

func printDNSPrerequisite(zoneName string) {
	fmt.Println("\n── Prerequisite: DNS API token (workspace) ──")
	fmt.Println("  Wrangler login cannot create or edit DNS records.")
	fmt.Println("  One Cloudflare API token is shared across all apps in this workspace.")
	fmt.Println("  Stored under: .cogs/cloudflare/ (not per-app .env)")
	fmt.Println("  Required permission:")
	fmt.Println("    • Zone → DNS → Edit  (include zone: " + zoneName + ")")
	fmt.Println()
	fmt.Println("  Steps:")
	fmt.Println("    1. Browser opens the Cloudflare “Create API token” page.")
	fmt.Println("    2. Start from template “Edit zone DNS”, limit to this zone.")
	fmt.Println("    3. Create token, copy it, paste here.")
	fmt.Println()
	fmt.Println("  Or set COGS_CF_API_TOKEN in the environment before launch.")
}

func ensureLaunchDNSPrerequisite(projectDir, zoneName string) error {
	migrateLegacyTokenToWorkspace(projectDir, zoneName)
	token, err := cfTokenDNS(projectDir, zoneName)
	if err == nil && tokenCanEditDNS(token, zoneName) {
		fmt.Println("\n✓ DNS API token OK (can edit zone " + zoneName + ")")
		return nil
	}
	printDNSPrerequisite(zoneName)
	if !stdinIsInteractive() {
		return fmt.Errorf("missing DNS-capable token: add one under .cogs/cloudflare/ or set COGS_CF_API_TOKEN, then re-run cogs launch")
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
	wsRoot, _, err := workspace.FindRoot(projectDir)
	if err != nil {
		return fmt.Errorf("workspace root: %w", err)
	}
	if err := workspace.SaveDNSToken(wsRoot, zoneName, pasted); err != nil {
		return err
	}
	fmt.Println("Saved DNS token to workspace (.cogs/cloudflare/) — reused by all apps")
	return nil
}

func stdinIsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
