package hostcheck

import (
	"fmt"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// PageHostname builds the public UI hostname (apex when subdomain is empty).
func PageHostname(pageSub, zone string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return ""
	}
	pageSub = strings.TrimSpace(pageSub)
	if pageSub == "" {
		return zone
	}
	return pageSub + "." + zone
}

// ValidateSubdomain checks DNS label rules when non-empty.
func ValidateSubdomain(sub string) error {
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return nil
	}
	if len(sub) > 63 {
		return fmt.Errorf("subdomain too long (max 63 characters)")
	}
	for i, r := range sub {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if i == 0 || i == len(sub)-1 {
			ok = ok && r != '-'
		}
		if !ok {
			return fmt.Errorf("subdomain %q has invalid characters (use a-z, 0-9, hyphen)", sub)
		}
	}
	return nil
}

// FindPageHostConflict returns an app name already using the same page hostname.
func FindPageHostConflict(wsRoot, pageSub, zone, skipApp string) (string, error) {
	want := strings.ToLower(PageHostname(pageSub, zone))
	if want == "" {
		return "", nil
	}
	skipApp = strings.TrimSpace(skipApp)
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return "", err
	}
	for _, a := range ws.Apps {
		if a.Name == skipApp {
			continue
		}
		appDir, err := workspace.ResolveApp(wsRoot, ws, a.Name)
		if err != nil {
			continue
		}
		cfg, err := appcfg.Load(appDir)
		if err != nil {
			continue
		}
		if host := strings.ToLower(cfg.PageHost()); host != "" && host == want {
			return a.Name, nil
		}
	}
	return "", nil
}

// FindHostConflicts checks UI (and split API) hostnames against all apps.
func FindHostConflicts(wsRoot, pageSub, zone, skipApp string) ([]string, error) {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return nil, nil
	}
	skipApp = strings.TrimSpace(skipApp)
	wantPage := strings.ToLower(PageHostname(pageSub, zone))
	var conflicts []string
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return nil, err
	}
	for _, a := range ws.Apps {
		if a.Name == skipApp {
			continue
		}
		appDir, err := workspace.ResolveApp(wsRoot, ws, a.Name)
		if err != nil {
			continue
		}
		cfg, err := appcfg.Load(appDir)
		if err != nil {
			continue
		}
		if wantPage != "" {
			if h := strings.ToLower(cfg.PageHost()); h != "" && h == wantPage {
				conflicts = append(conflicts, fmt.Sprintf("UI hostname %s used by %q", h, a.Name))
			}
		}
	}
	return conflicts, nil
}
