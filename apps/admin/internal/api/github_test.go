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

func TestReposPublicLookupQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	repos := publicReposOnly(ctx, "irgipaulius/cogs")
	if len(repos) != 1 {
		t.Fatalf("want 1 public repo, got %d", len(repos))
	}
	if repos[0].FullName != "irgipaulius/cogs" {
		t.Fatalf("full_name=%q", repos[0].FullName)
	}
}

func TestAddAppWithoutGhAcceptsPublicRepoAttempt(t *testing.T) {
	dir := t.TempDir()
	s := &Server{WorkspaceRoot: dir}
	user := &adminstore.EffectiveUser{IsRoot: true}
	req := httptest.NewRequest(http.MethodPost, "/api/apps", strings.NewReader(`{"repo":"definitely-not-a-real-owner-xyz/fake-repo-99999"}`))
	rec := httptest.NewRecorder()
	s.handleAddApp(rec, req, user)
	if rec.Code == http.StatusBadRequest {
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if strings.Contains(body["error"], "GitHub not logged in") {
			t.Fatalf("should not require gh login for install attempt: %q", body["error"])
		}
	}
}
