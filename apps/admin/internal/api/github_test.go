package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/irgipaulius/cogs/pkg/adminstore"
)

func TestGitHubLoginStartMissingClientIDReturnsJSON400(t *testing.T) {
	t.Setenv("COGS_GITHUB_CLIENT_ID", "")
	s := &Server{WorkspaceRoot: t.TempDir()}
	user := &adminstore.EffectiveUser{IsRoot: true, Username: "root"}
	req := httptest.NewRequest(http.MethodPost, "/api/github/login/start", nil)
	rec := httptest.NewRecorder()
	s.handleGitHubLoginStart(rec, req, user)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type=%q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] == "" {
		t.Fatalf("expected error message, got %v", body)
	}
	if strings.HasPrefix(body["error"], "<!DOCTYPE") {
		t.Fatalf("unexpected HTML error")
	}
}

func TestGitHubStatusWithoutSupervisor(t *testing.T) {
	dir := t.TempDir()
	s := &Server{WorkspaceRoot: dir}
	user := &adminstore.EffectiveUser{IsRoot: true}
	req := httptest.NewRequest(http.MethodGet, "/api/github/status", nil)
	rec := httptest.NewRecorder()
	s.handleGitHubStatus(rec, req, user)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var st struct {
		SignedIn bool `json:"signed_in"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.SignedIn {
		t.Fatal("expected signed_out")
	}
}

func TestAddAppRequiresGitHub(t *testing.T) {
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
