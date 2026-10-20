package cli

import (
	"net/url"
	"os"
	"strings"
)

// hostsFromEnv returns page and API hostnames from an existing .env when set.
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
	return pageHost, apiHost, true
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
