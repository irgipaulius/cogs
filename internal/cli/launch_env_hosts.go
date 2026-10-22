package cli

import (
	"net"
	"net/url"
	"os"
	"strings"
)

// hostsFromEnv returns page and API hostnames from an existing .env when set.
func listenFromEnv(envPath string) (listen string, ok bool) {
	b, err := os.ReadFile(envPath)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COGS_LISTEN=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "COGS_LISTEN=")), true
		}
	}
	return "", false
}

func hostsFromEnv(envPath string) (pageHost, apiHost string, ok bool) {
	b, err := os.ReadFile(envPath)
	if err != nil {
		return "", "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COGS_PUBLIC_PAGE_ORIGIN=") {
			pageHost = hostFromEnvURL(strings.TrimPrefix(line, "COGS_PUBLIC_PAGE_ORIGIN="))
		}
		if strings.HasPrefix(line, "COGS_PUBLIC_API_URL=") {
			apiHost = hostFromEnvURL(strings.TrimPrefix(line, "COGS_PUBLIC_API_URL="))
		}
	}
	if pageHost == "" || apiHost == "" {
		return "", "", false
	}
	if !isPublicLaunchHost(pageHost) || !isPublicLaunchHost(apiHost) {
		return "", "", false
	}
	return pageHost, apiHost, true
}

// isPublicLaunchHost rejects LAN URLs mistakenly stored in COGS_PUBLIC_* (e.g. http://192.168.x.x:port).
func isPublicLaunchHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	h := host
	if strings.Contains(h, ":") {
		h, _, _ = strings.Cut(h, ":")
	}
	if ip := net.ParseIP(h); ip != nil {
		return false
	}
	if h == "localhost" || strings.HasSuffix(h, ".local") {
		return false
	}
	return strings.Contains(host, ".")
}

func hostFromEnvURL(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		raw = strings.TrimPrefix(raw, "https://")
		raw = strings.TrimPrefix(raw, "http://")
		return strings.TrimSuffix(raw, "/")
	}
	return u.Host
}

func zoneFromPageHost(pageHost string) string {
	return zoneNameFromHost(pageHost)
}
