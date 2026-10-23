package cfprovision

import (
	"strings"

	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/state"
)

// Hosts resolves public page and API hostnames from desired state + manifest.
func Hosts(app state.AppView, m launch.Manifest) (pageHost, apiHost string) {
	zone := strings.TrimSpace(app.Zone)
	if zone == "" {
		zone = strings.TrimSpace(m.Zone)
	}
	pageSub := strings.TrimSpace(app.PageSubdomain)
	if pageSub == "" && app.PageSubdomain == "" && zone != "" {
		pageSub = strings.TrimSpace(m.PageSubdomain)
	}
	apiSub := strings.TrimSpace(app.APISubdomain)
	if apiSub == "" {
		apiSub = strings.TrimSpace(m.APISubdomain)
	}
	if zone == "" {
		return "", ""
	}
	if pageSub == "" {
		pageHost = zone
	} else {
		pageHost = pageSub + "." + zone
	}
	if m.IsCombined() {
		apiHost = pageHost
	} else if apiSub != "" {
		apiHost = apiSub + "." + zone
	}
	return pageHost, apiHost
}
