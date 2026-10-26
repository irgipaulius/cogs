package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/irgipaulius/cogs/pkg/adminstore"
)

func TestBackupsListRequiresRoot(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/apps/todo/backups", nil)
	req.SetPathValue("name", "todo")
	w := httptest.NewRecorder()
	s.handleAppBackupsList(w, req, &adminstore.EffectiveUser{IsRoot: false})
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestBackupsCreateRequiresRoot(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/apps/todo/backups", nil)
	req.SetPathValue("name", "todo")
	w := httptest.NewRecorder()
	s.handleAppBackupsCreate(w, req, &adminstore.EffectiveUser{IsRoot: false})
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestBackupsListUnauthorized(t *testing.T) {
	st, err := adminstore.Open(t.TempDir() + "/admin.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := &Server{Store: st}
	req := httptest.NewRequest(http.MethodGet, "/api/apps/todo/backups", nil)
	req.SetPathValue("name", "todo")
	w := httptest.NewRecorder()
	s.auth(s.handleAppBackupsList)(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestBackupRestoreRequiresRoot(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/apps/todo/backups/1/restore", nil)
	req.SetPathValue("name", "todo")
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	s.handleAppBackupRestore(w, req, &adminstore.EffectiveUser{IsRoot: false})
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d", w.Code)
	}
}
