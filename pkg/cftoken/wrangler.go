package cftoken

import (
	"fmt"
	"os"
	"regexp"

	"github.com/irgipaulius/cogs/pkg/wranglerpaths"
)

// WranglerConfigPath returns wrangler default.toml (wrangler 4 resolution order).
func WranglerConfigPath() (string, error) {
	return wranglerpaths.ConfigDefaultTOML()
}

// WranglerOAuthToken reads oauth/api token from the config path wrangler would use.
func WranglerOAuthToken() (string, error) {
	path, err := WranglerConfigPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read wrangler config %s: %w", path, err)
	}
	re := regexp.MustCompile(`(?m)^oauth_token\s*=\s*"([^"]+)"`)
	m := re.FindSubmatch(b)
	if len(m) < 2 {
		re2 := regexp.MustCompile(`(?m)^api_token\s*=\s*"([^"]+)"`)
		m = re2.FindSubmatch(b)
	}
	if len(m) < 2 {
		return "", fmt.Errorf("no oauth_token in %s", path)
	}
	return string(m[1]), nil
}
