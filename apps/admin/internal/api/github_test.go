package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/ghcli"
)

func TestAuthStatusJSONShape(t *testing.T) {
	dir := t.TempDir()
	s := &Server{WorkspaceRoot: dir}
	user := &adminstore.EffectiveUser{IsRoot: true}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	s.handleAuthStatus(rec, req, user)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var st struct {
		GitHub     map[string]any `json:"github"`
		Cloudflare map[string]any `json:"cloudflare"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.GitHub == nil || st.Cloudflare == nil {
		t.Fatalf("missing keys: %v", st)
	}
}

func TestAddAppRequiresGitHubWhenGhLoggedOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gh, err := ghcli.Status(ctx)
	if err != nil {
		t.Skip(err)
	}
	if gh.LoggedIn {
		t.Skip("gh logged in on this machine — skip logged-out test")
	}
	dir := t.TempDir()
	s := &Server{WorkspaceRoot: dir}
	user := &adminstore.EffectiveUser{IsRoot: true}
	req := httptest.NewRequest(http.MethodPost, "/api/apps", strings.NewReader(`{"name":"x","repo":"a/b"}`))
	rec := httptest.NewRecorder()
	s.handleAddApp(rec, req, user)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %s", rec.Body.String())
	}
	if !strings.Contains(body["error"], "GitHub") {
		t.Fatalf("error=%q", body["error"])
	}
}
