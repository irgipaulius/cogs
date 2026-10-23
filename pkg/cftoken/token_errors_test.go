package cftoken

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForReportsWranglerRejectionReason(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COGS_HOME", home)
	t.Setenv("COGS_CF_API_TOKEN", "")
	_ = os.MkdirAll(filepath.Join(home, ".wrangler"), 0o755)
	r := Resolver{OAuth: func() (string, error) {
		return "", formatNoTokenError("/home/cogs/.config/.wrangler/config/default.toml", []string{"/home/cogs/.wrangler/config/default.toml (missing)"}, nil)
	}}
	_, err := r.For(context.Background(), ScopeAPI)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "no Cloudflare token") || !strings.Contains(msg, ".config/.wrangler") {
		t.Fatalf("unexpected: %q", msg)
	}
}
