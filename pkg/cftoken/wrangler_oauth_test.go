package cftoken

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestRefreshExpiredOAuthToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/token" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "old-refresh" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fresh-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer srv.Close()

	// Patch token URL for test via direct refresh on a temp config file.
	dir := t.TempDir()
	cfg := filepath.Join(dir, "default.toml")
	expired := time.Now().UTC().Add(-1 * time.Hour).Format("2006-01-02T15:04:05.000Z")
	if err := os.WriteFile(cfg, []byte(`oauth_token = "expired"
refresh_token = "old-refresh"
expiration_time = "`+expired+`"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	oldURL := oauthTokenURL
	oauthTokenURL = srv.URL + "/oauth2/token"
	t.Cleanup(func() { oauthTokenURL = oldURL })

	if err := refreshWranglerOAuth(cfg); err != nil {
		t.Fatal(err)
	}
	creds, err := readWranglerCreds(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if creds.OAuthToken != "fresh-access" {
		t.Fatalf("token=%q want fresh-access", creds.OAuthToken)
	}
	if creds.RefreshToken != "new-refresh" {
		t.Fatalf("refresh=%q want new-refresh", creds.RefreshToken)
	}
}

func TestMaintainOAuthAcrossTwoExpiries(t *testing.T) {
	refreshes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshes++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok-` + strconv.Itoa(refreshes) + `","refresh_token":"rt","expires_in":3600}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COGS_HOME", home)
	cfg := filepath.Join(home, ".config", ".wrangler", "config", "default.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	writeCfg := func(exp time.Time) {
		body := `oauth_token = "old"
refresh_token = "rt"
expiration_time = "` + exp.Format("2006-01-02T15:04:05.000Z") + `"
`
		_ = os.WriteFile(cfg, []byte(body), 0o600)
	}
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	SetWranglerNowForTests(func() time.Time { return clock })
	t.Cleanup(func() { SetWranglerNowForTests(nil) })

	oldURL := oauthTokenURL
	oauthTokenURL = srv.URL + "/oauth2/token"
	t.Cleanup(func() { oauthTokenURL = oldURL })

	writeCfg(clock.Add(-30 * time.Minute))
	if _, err := wranglerOAuthToken(false); err != nil {
		t.Fatal(err)
	}
	if refreshes != 1 {
		t.Fatalf("refreshes=%d want 1", refreshes)
	}
	clock = clock.Add(55 * time.Minute)
	writeCfg(clock.Add(-30 * time.Minute))
	if _, err := wranglerOAuthToken(false); err != nil {
		t.Fatal(err)
	}
	if refreshes != 2 {
		t.Fatalf("refreshes=%d want 2 after second expiry window", refreshes)
	}
}
