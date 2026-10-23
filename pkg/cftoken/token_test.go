package cftoken

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/workspace"
)

func TestQueueScopePrefersQueueToken(t *testing.T) {
	t.Setenv("COGS_CF_QUEUE_API_TOKEN", "queue-tok")
	t.Setenv("COGS_CF_API_TOKEN", "api-tok")
	r := Resolver{}
	got, err := r.For(context.Background(), ScopeQueue)
	if err != nil {
		t.Fatal(err)
	}
	if got != "queue-tok" {
		t.Fatalf("got %q want queue-tok", got)
	}
}

func TestAPIScopeSkipsDNSTokenUsesOAuth(t *testing.T) {
	dir := t.TempDir()
	if err := workspace.SaveDNSToken(dir, "example.com", "dns-only-token"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COGS_CF_API_TOKEN", "")
	r := Resolver{
		WSRoot: dir,
		OAuth: func() (string, error) {
			return "oauth-session-token", nil
		},
	}
	got, err := r.For(context.Background(), ScopeAPI)
	if err != nil {
		t.Fatal(err)
	}
	if got != "oauth-session-token" {
		t.Fatalf("API scope got %q want OAuth token (not DNS-only %q)", got, "dns-only-token")
	}
	got, err = r.For(context.Background(), ScopeDNS)
	if err != nil {
		t.Fatal(err)
	}
	if got != "dns-only-token" {
		t.Fatalf("DNS scope got %q want dns-only-token", got)
	}
	_ = filepath.Join(dir, ".cogs")
}

func TestQueueScopeSkipsDNSTokenUsesOAuthWhenNoQueueEnv(t *testing.T) {
	dir := t.TempDir()
	if err := workspace.SaveDNSToken(dir, "z.example", "dns-only"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COGS_CF_QUEUE_API_TOKEN", "")
	t.Setenv("COGS_CF_API_TOKEN", "")
	r := Resolver{
		WSRoot: dir,
		OAuth: func() (string, error) { return "oauth-q", nil },
	}
	got, err := r.For(context.Background(), ScopeQueue)
	if err != nil {
		t.Fatal(err)
	}
	if got != "oauth-q" {
		t.Fatalf("queue scope got %q want oauth", got)
	}
}
