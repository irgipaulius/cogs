package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/gitpublic"
)

func TestRepoPreviewBadQuery(t *testing.T) {
	s := &Server{}
	user := &adminstore.EffectiveUser{IsRoot: true}
	req := httptest.NewRequest(http.MethodGet, "/api/repos/preview?q=not-valid", nil)
	rec := httptest.NewRecorder()
	s.handleRepoPreview(rec, req, user)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRepoPreviewPublicCOGS(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	prev, err := gitpublic.PreviewRemote(ctx, "irgipaulius/cogs")
	if err != nil {
		t.Fatal(err)
	}
	if prev.State != gitpublic.StateReady {
		t.Fatalf("state=%q msg=%q", prev.State, prev.Message)
	}
	if prev.App == nil || prev.App.Name == "" {
		t.Fatalf("missing app: %+v", prev.App)
	}
}

func TestRepoPreviewHandlerShape(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	dir := t.TempDir()
	s := &Server{WorkspaceRoot: dir}
	user := &adminstore.EffectiveUser{IsRoot: true}
	req := httptest.NewRequest(http.MethodGet, "/api/repos/preview?q=irgipaulius/cogs", nil)
	rec := httptest.NewRecorder()
	s.handleRepoPreview(rec, req, user)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Preview gitpublic.RepoPreview `json:"preview"`
		Zones   []string              `json:"zones"`
		Hosts   map[string]string     `json:"hosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Preview.State != gitpublic.StateReady {
		t.Fatalf("preview state=%q", body.Preview.State)
	}
}
