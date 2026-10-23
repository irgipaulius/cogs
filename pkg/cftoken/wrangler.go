package cftoken

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/irgipaulius/cogs/pkg/hostenv"
)

// WranglerOAuthToken reads the OAuth token from wrangler's config on the host.
func WranglerOAuthToken() (string, error) {
	path, err := wranglerConfigPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read wrangler config: %w", err)
	}
	re := regexp.MustCompile(`(?m)^oauth_token\s*=\s*"([^"]+)"`)
	m := re.FindSubmatch(b)
	if len(m) < 2 {
		re2 := regexp.MustCompile(`(?m)^api_token\s*=\s*"([^"]+)"`)
		m = re2.FindSubmatch(b)
	}
	if len(m) < 2 {
		return "", fmt.Errorf("no oauth_token in %s; run wrangler login", path)
	}
	return string(m[1]), nil
}

func wranglerConfigPath() (string, error) {
	if h := strings.TrimSpace(os.Getenv("WRANGLER_HOME")); h != "" {
		return filepath.Join(h, "config", "default.toml"), nil
	}
	home := hostenv.ServiceHome()
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Preferences", ".wrangler", "config", "default.toml"), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, ".wrangler", "config", "default.toml"), nil
	}
	return filepath.Join(home, ".config", ".wrangler", "config", "default.toml"), nil
}
