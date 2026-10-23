package cftoken

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
