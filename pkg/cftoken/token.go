package cftoken

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/irgipaulius/cogs/pkg/workspace"
)

// Scope selects token capabilities hint (all use same bearer for now).
type Scope string

const (
	ScopeDNS   Scope = "dns"
	ScopeAPI   Scope = "api"
	ScopeQueue Scope = "queue"
)

// Resolver resolves Cloudflare API tokens.
type Resolver struct {
	WSRoot string
	// OAuth is set by CLI wiring (wrangler session).
	OAuth func() (string, error)
}

// For returns a bearer token for Cloudflare API calls.
func (r *Resolver) For(ctx context.Context, scope Scope) (string, error) {
	_ = ctx
	switch scope {
	case ScopeQueue:
		if t := strings.TrimSpace(os.Getenv("COGS_CF_QUEUE_API_TOKEN")); t != "" {
			return t, nil
		}
		if t := strings.TrimSpace(os.Getenv("COGS_CF_API_TOKEN")); t != "" {
			return t, nil
		}
	case ScopeDNS:
		if t := strings.TrimSpace(os.Getenv("COGS_CF_API_TOKEN")); t != "" {
			return t, nil
		}
		if r.WSRoot != "" {
			if t, err := workspace.DNSTokenForZone(r.WSRoot, ""); err == nil && t != "" {
				return t, nil
			}
		}
	case ScopeAPI:
		if t := strings.TrimSpace(os.Getenv("COGS_CF_API_TOKEN")); t != "" {
			return t, nil
		}
	default:
		if t := strings.TrimSpace(os.Getenv("COGS_CF_API_TOKEN")); t != "" {
			return t, nil
		}
	}
	var reasons []string
	if r.OAuth != nil {
		if t, err := r.OAuth(); err == nil && t != "" {
			return t, nil
		} else if err != nil {
			reasons = append(reasons, err.Error())
		}
	}
	if t, err := WranglerOAuthToken(); err == nil && t != "" {
		return t, nil
	} else if err != nil {
		reasons = append(reasons, err.Error())
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "set COGS_CF_API_TOKEN or run wrangler login")
	}
	return "", fmt.Errorf("no Cloudflare token — %s", strings.Join(reasons, "; "))
}
