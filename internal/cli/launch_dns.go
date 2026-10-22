package cli

import (
	"fmt"

	"github.com/irgipaulius/cogs/pkg/launch"
)

// configureSplitPublicHosts sets tunnel ingress + DNS for the API hostname only.
// UI hostname is attached via Worker custom_domain routes (wrangler deploy), not Pages.
func configureSplitPublicHosts(projectDir string, m launch.Manifest, apiHost string) error {
	localSvc := "http://" + listenAddr(m.BindHost, m.Port)
	if err := ensureTunnelIngress(projectDir, m.TunnelName, apiHost, localSvc); err != nil {
		return fmt.Errorf("tunnel ingress: %w", err)
	}
	if err := ensureTunnelDNS(projectDir, m.TunnelName, apiHost); err != nil {
		return err
	}
	return nil
}

func configureCombinedHost(projectDir string, m launch.Manifest, host string) error {
	localSvc := "http://" + listenAddr(m.BindHost, m.Port)
	if err := ensureTunnelIngress(projectDir, m.TunnelName, host, localSvc); err != nil {
		return fmt.Errorf("tunnel ingress: %w", err)
	}
	return ensureTunnelDNS(projectDir, m.TunnelName, host)
}
