package cli

import (
	"fmt"

	"github.com/irgipaulius/cogs/pkg/launch"
)

func configurePublicHosts(projectDir string, m launch.Manifest, pageHost, apiHost string) error {
	localSvc := fmt.Sprintf("http://127.0.0.1:%d", m.Port)
	if err := ensureTunnelIngress(projectDir, m.TunnelName, apiHost, localSvc); err != nil {
		return fmt.Errorf("tunnel ingress: %w", err)
	}
	if err := ensureTunnelDNS(projectDir, m.TunnelName, apiHost); err != nil {
		return err
	}
	if err := ensurePagesCustomDomain(projectDir, m.PagesProject, pageHost); err != nil {
		return fmt.Errorf("pages domain: %w", err)
	}
	return nil
}
