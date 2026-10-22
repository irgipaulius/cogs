package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/gitpublic"
	"github.com/irgipaulius/cogs/pkg/hostcheck"
	"github.com/irgipaulius/cogs/pkg/installjob"
)

func (s *Server) handleAddApp(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	if s.WorkspaceRoot == "" {
		writeAPIError(w, http.StatusServiceUnavailable, "workspace root not configured on admin server")
		return
	}
	var body struct {
		Name          string `json:"name"`
		Repo          string `json:"repo"`
		DeployMode    string `json:"deploy_mode"`
		PageSubdomain string `json:"page_subdomain"`
		PageApex      bool   `json:"page_apex"`
		Zone          string `json:"zone"`
		ScaffoldOnly  bool   `json:"scaffold_only"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "bad json")
		return
	}
	pageSub := strings.TrimSpace(body.PageSubdomain)
	pageApex := body.PageApex
	if pageApex {
		pageSub = ""
	}
	if err := hostcheck.ValidateSubdomain(pageSub); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if zone := strings.TrimSpace(body.Zone); zone != "" {
		if conflict, err := hostcheck.FindPageHostConflict(s.WorkspaceRoot, pageSub, zone, strings.TrimSpace(body.Name)); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err.Error())
			return
		} else if conflict != "" {
			writeAPIError(w, http.StatusConflict, "hostname already used by app "+conflict)
			return
		}
	}

	mgr := s.ensureInstalls()
	req := installjob.Request{
		RepoFull:      gitpublic.NormalizeFullName(body.Repo),
		AppName:       strings.TrimSpace(body.Name),
		DeployMode:    body.DeployMode,
		PageSubdomain: pageSub,
		PageApex:      pageApex,
		Zone:          strings.TrimSpace(body.Zone),
		ScaffoldOnly:  body.ScaffoldOnly,
	}
	job, err := mgr.Create(r.Context(), req)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.Store != nil {
		_ = s.Store.WriteAudit(r.Context(), user, "install_start", job.ID+": "+req.RepoFull)
	}
	writeJSON(w, map[string]any{
		"install_id": job.ID,
		"app_name":   job.AppName,
		"status":     job.Status,
	})
}
