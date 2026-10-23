// Package wranglerpaths resolves wrangler config locations the same way wrangler 4 does.
package wranglerpaths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/irgipaulius/cogs/pkg/hostenv"
)

// ConfigDefaultTOML returns the path wrangler uses for default.toml.
func ConfigDefaultTOML() (string, error) {
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
	legacy := filepath.Join(home, ".wrangler")
	if st, err := os.Stat(legacy); err == nil && st.IsDir() {
		return filepath.Join(legacy, "config", "default.toml"), nil
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, ".wrangler", "config", "default.toml"), nil
	}
	return filepath.Join(home, ".config", ".wrangler", "config", "default.toml"), nil
}

// LegacyHomeDir is $HOME/.wrangler when present (wrangler 4 legacy layout).
func LegacyHomeDir() string {
	home := hostenv.ServiceHome()
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return ""
	}
	p := filepath.Join(home, ".wrangler")
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return p
	}
	return ""
}

// ModernConfigTOML is the XDG-style config path (usually holds real login).
func ModernConfigTOML() (string, error) {
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
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, ".wrangler", "config", "default.toml"), nil
	}
	return filepath.Join(home, ".config", ".wrangler", "config", "default.toml"), nil
}

// ShadowDiag reports when legacy ~/.wrangler shadows credentials in the modern path.
func ShadowDiag() (shadowing bool, message string) {
	legacy := LegacyHomeDir()
	if legacy == "" {
		return false, ""
	}
	active, _ := ConfigDefaultTOML()
	modern, _ := ModernConfigTOML()
	if active == modern {
		return false, ""
	}
	hasCred := func(p string) bool {
		b, err := os.ReadFile(p)
		if err != nil {
			return false
		}
		s := string(b)
		return strings.Contains(s, "oauth_token") || strings.Contains(s, "api_token")
	}
	if hasCred(active) {
		return false, ""
	}
	if !hasCred(modern) {
		return true, "Legacy " + legacy + " exists without credentials and shadows wrangler config — remove it or add credentials there"
	}
	return true, "Legacy " + legacy + " shadows wrangler login at " + modern + " — remove " + legacy + " or run wrangler login with WRANGLER_HOME unset"
}
