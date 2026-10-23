package cftoken

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Regression: queue pull used a stale OAuth token hourly on b5c9b82; refresh before expiry must keep API calls working.
func TestQueueScopeTokenRefreshesBeforeHourlyExpiry(t *testing.T) {
	refreshes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshes++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"queue-tok","refresh_token":"rt","expires_in":3600}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COGS_HOME", home)
	cfg := filepath.Join(home, ".config", ".wrangler", "config", "default.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	SetWranglerNowForTests(func() time.Time { return clock })
	t.Cleanup(func() { SetWranglerNowForTests(nil) })

	oldURL := oauthTokenURL
	oauthTokenURL = srv.URL + "/oauth2/token"
	t.Cleanup(func() { oauthTokenURL = oldURL })

	write := func(exp time.Time) {
		body := `oauth_token = "stale"
refresh_token = "rt"
expiration_time = "` + exp.Format("2006-01-02T15:04:05.000Z") + `"
`
		_ = os.WriteFile(cfg, []byte(body), 0o600)
	}
	write(clock.Add(-30 * time.Minute))
	tok, err := (&Resolver{}).For(context.Background(), ScopeQueue)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "queue-tok" || refreshes != 1 {
		t.Fatalf("token=%q refreshes=%d want queue-tok/1", tok, refreshes)
	}
	// Simulate hourly pull tick: still expired relative to wall clock, must refresh again.
	clock = clock.Add(55 * time.Minute)
	write(clock.Add(-30 * time.Minute))
	MaintainWranglerOAuth()
	tok2, err := (&Resolver{}).For(context.Background(), ScopeQueue)
	if err != nil {
		t.Fatal(err)
	}
	if tok2 != "queue-tok" || refreshes < 2 {
		t.Fatalf("second token=%q refreshes=%d want >=2", tok2, refreshes)
	}
}
