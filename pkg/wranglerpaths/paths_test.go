package wranglerpaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigSkipsEmptyLegacyForModern(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COGS_HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	legacyDir := filepath.Join(home, ".wrangler", "config")
	_ = os.MkdirAll(legacyDir, 0o755)
	// Empty legacy dir exists but no default.toml — must not win.

	modernDir := filepath.Join(home, ".config", ".wrangler", "config")
	_ = os.MkdirAll(modernDir, 0o755)
	modern := filepath.Join(modernDir, "default.toml")
	if err := os.WriteFile(modern, []byte(`oauth_token = "live-token"
refresh_token = "refresh"
expiration_time = "2099-01-01T00:00:00.000Z"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, _, err := ResolveConfigDefaultTOML()
	if err != nil {
		t.Fatal(err)
	}
	if got != modern {
		t.Fatalf("config path=%q want modern %q", got, modern)
	}
}
