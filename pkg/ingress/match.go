package ingress

import (
	"net/http"
	"strings"
)

func (c *Config) routeAllowsDirect(r *http.Request) bool {
	for _, rule := range c.Routes {
		if rule.AllowDirect && rule.matches(r) {
			return true
		}
	}
	return false
}

func (r Route) matches(req *http.Request) bool {
	pat := strings.TrimSpace(r.Pattern)
	if pat == "" {
		return false
	}
	method := req.Method
	path := req.URL.Path
	if i := strings.IndexByte(pat, ' '); i >= 0 {
		method = strings.ToUpper(strings.TrimSpace(pat[:i]))
		pat = strings.TrimSpace(pat[i+1:])
	} else {
		method = ""
	}
	if method != "" && !strings.EqualFold(method, req.Method) {
		return false
	}
	if strings.HasSuffix(pat, "*") {
		prefix := strings.TrimSuffix(pat, "*")
		return strings.HasPrefix(path, prefix)
	}
	return path == pat
}

func (c *Config) tunnelAllowed(r *http.Request) bool {
	if c.TrustLoopback && IsLoopbackPeer(r) {
		return true
	}
	return ViaCloudflare(r)
}

func (c *Config) directAllowed(r *http.Request) bool {
	if !c.routeAllowsDirect(r) {
		return false
	}
	return directSecretOK(r, c.DirectSecret)
}
