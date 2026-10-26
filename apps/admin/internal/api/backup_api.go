package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/backup"
	"github.com/irgipaulius/cogs/pkg/hostenv"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func (s *Server) handleAppBackupsList(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	name := r.PathValue("name")
	st, _, err := s.ensureState()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, err := st.ListBackups(name, 50)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type row struct {
		ID        int64  `json:"id"`
		CreatedAt string `json:"created_at"`
		Bytes     int64  `json:"bytes"`
		SHA256    string `json:"sha256"`
		Driver    string `json:"driver"`
	}
	out := make([]row, 0, len(rows))
	for _, b := range rows {
		out = append(out, row{
			ID: b.ID, CreatedAt: b.CreatedAt.UTC().Format(time.RFC3339),
			Bytes: b.Bytes, SHA256: b.SHA256, Driver: b.Driver,
		})
	}
	writeJSON(w, out)
}

func (s *Server) handleAppBackupsCreate(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	st, _, err := s.ensureState()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, ws, err := workspace.FindRoot(s.WorkspaceRoot)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	dir, err := workspace.ResolveApp(s.WorkspaceRoot, ws, name)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	svc := &backup.Service{WSRoot: s.WorkspaceRoot, Store: st}
	id, err := svc.Snapshot(r.Context(), name, dir)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"id": id})
}

func (s *Server) handleAppBackupRestore(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	name := r.PathValue("name")
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid id")
		return
	}
	cmd := hostenv.CogsCommandContext(r.Context(), s.WorkspaceRoot, "backup", "restore", name, fmt.Sprintf("%d", id), "--yes")
	if out, err := cmd.CombinedOutput(); err != nil {
		writeAPIError(w, http.StatusBadRequest, strings.TrimSpace(string(out)))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}
