package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/appremove"
	"github.com/irgipaulius/cogs/pkg/installjob"
)

func (s *Server) handleRemoveApp(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" || name == "admin" {
		writeAPIError(w, http.StatusBadRequest, "invalid app")
		return
	}
	var body struct {
		RemoveClone bool `json:"remove_clone"`
	}
	_ = jsonDecodeOptional(r, &body)
	detail, err := appremove.Remove(r.Context(), appremove.Options{
		WSRoot:       s.WorkspaceRoot,
		AppName:      name,
		RemoveClone:  body.RemoveClone,
		RemoveGoWork: true,
	})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.Store != nil {
		_ = s.Store.WriteAudit(r.Context(), user, "remove_app", name+": "+detail)
	}
	writeJSON(w, map[string]string{"detail": detail})
}

func (s *Server) handleFinishInstall(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	mgr := s.ensureInstalls()
	job, err := mgr.Create(r.Context(), installjob.Request{FinishApp: name})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"install_id": job.ID, "app_name": job.AppName, "status": job.Status})
}

func jsonDecodeOptional(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
