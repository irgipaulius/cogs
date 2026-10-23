// Package wranglerpaths resolves wrangler config locations the same way wrangler 4 does.
package wranglerpaths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/irgipaulius/cogs/pkg/hostenv"
)

// ConfigDefaultTOML returns the default.toml path that actually contains credentials.
func ConfigDefaultTOML() (string, error) {
	path, _, err := ResolveConfigDefaultTOML()
	return path, err
}

// ResolveConfigDefaultTOML picks the first candidate default.toml with oauth/api token.
// logHint is a short note for errors (which paths were tried).
func ResolveConfigDefaultTOML() (path string, tried []string, err error) {
	for _, c := range configCandidates() {
		if !configHasCredentials(c) {
			if _, statErr := os.Stat(c); statErr != nil {
				tried = append(tried, c+" (missing)")
			} else {
				tried = append(tried, c+" (no oauth_token or api_token)")
			}
			continue
		}
		return c, tried, nil
	}
	return "", tried, fmt.Errorf("no wrangler credentials in any config path")
}

func configCandidates() []string {
	if h := strings.TrimSpace(os.Getenv("WRANGLER_HOME")); h != "" {
		return []string{filepath.Join(h, "config", "default.toml")}
	}
	home := hostenv.ServiceHome()
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return nil
	}
	var out []string
	if runtime.GOOS == "darwin" {
		out = append(out, filepath.Join(home, "Library", "Preferences", ".wrangler", "config", "default.toml"))
	}
	legacy := filepath.Join(home, ".wrangler", "config", "default.toml")
	modern := modernConfigPath(home)
	out = append(out, legacy, modern)
	// De-dupe while preserving order.
	seen := map[string]bool{}
	var uniq []string
	for _, p := range out {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		uniq = append(uniq, p)
	}
	return uniq
}

func modernConfigPath(home string) string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, ".wrangler", "config", "default.toml")
	}
	return filepath.Join(home, ".config", ".wrangler", "config", "default.toml")
}

func configHasCredentials(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := string(b)
	return strings.Contains(s, "oauth_token") || strings.Contains(s, "api_token")
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
	return modernConfigPath(home), nil
}

// ShadowDiag reports when legacy ~/.wrangler shadows credentials in the modern path.
func ShadowDiag() (shadowing bool, message string) {
	legacy := LegacyHomeDir()
	if legacy == "" {
		return false, ""
	}
	active, _, _ := ResolveConfigDefaultTOML()
	modern, _ := ModernConfigTOML()
	if active == modern || active == "" {
		return false, ""
	}
	hasCred := func(p string) bool {
		return configHasCredentials(p)
	}
	if hasCred(active) {
		return false, ""
	}
	if !hasCred(modern) {
		return true, "Legacy " + legacy + " exists without credentials and shadows wrangler config — remove it or add credentials there"
	}
	return true, "Legacy " + legacy + " shadows wrangler login at " + modern + " — remove " + legacy + " or run wrangler login with WRANGLER_HOME unset"
}
