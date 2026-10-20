package cockpit

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the best-effort client address (Cloudflare / proxy aware).
func ClientIP(r *http.Request) string {
	for _, h := range []string{"CF-Connecting-IP", "True-Client-IP", "X-Real-IP"} {
		if ip := strings.TrimSpace(r.Header.Get(h)); ip != "" {
			return ip
		}
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			ip := strings.TrimSpace(part)
			if ip != "" && !IsLikelyInternal(ip) {
				return ip
			}
		}
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// ClientCountry returns ISO 3166-1 alpha-2 from Cloudflare CF-IPCountry when the
// request is proxied through Cloudflare (tunnel, Workers, etc.). No IP geolocation
// is performed in CoGS; keys in telemetry by_country* use this same alpha-2 code.
func ClientCountry(r *http.Request) string {
	cc := strings.ToUpper(strings.TrimSpace(r.Header.Get("CF-IPCountry")))
	if cc == "" || cc == "XX" || cc == "T1" {
		return ""
	}
	return cc
}

// IsLikelyInternal is true for loopback, RFC1918, and common Docker bridge ranges.
func IsLikelyInternal(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	private := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
	}
	for _, cidr := range private {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
