package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/gitpublic"
	"github.com/irgipaulius/cogs/pkg/hostcheck"
	"github.com/irgipaulius/cogs/pkg/reconcile"
	"github.com/irgipaulius/cogs/pkg/state"
)

func (s *Server) handleAddApp(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	st, mgr, err := s.ensureState()
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, err.Error())
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
	if body.ScaffoldOnly {
		writeAPIError(w, http.StatusBadRequest, "scaffold_only not supported — use reconcile")
		return
	}
	repo := gitpublic.NormalizeFullName(body.Repo)
	if repo == "" {
		writeAPIError(w, http.StatusBadRequest, "repo required")
		return
	}
	appID := strings.TrimSpace(body.Name)
	pageSub := strings.TrimSpace(body.PageSubdomain)
	if body.PageApex {
		pageSub = ""
	}
	zone := strings.TrimSpace(body.Zone)
	if err := hostcheck.ValidateSubdomain(pageSub); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if zone != "" {
		if conflicts, err := hostcheck.FindHostConflicts(s.WorkspaceRoot, pageSub, zone, appID); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err.Error())
			return
		} else if len(conflicts) > 0 {
			writeAPIError(w, http.StatusConflict, conflicts[0])
			return
		}
		if err := st.CheckHostnames(appID, pageSub, "", zone); err != nil {
			writeAPIError(w, http.StatusConflict, err.Error())
			return
		}
	}
	if existing, _ := st.AppByRepo(repo); existing != "" && existing != appID {
		writeAPIError(w, http.StatusConflict, "repository already installed as "+existing)
		return
	}
	if appID == "" {
		appID = repoBaseName(repo)
	}
	mode := strings.TrimSpace(body.DeployMode)
	if mode == "" {
		mode = "push"
	}
	d := state.AppDesired{
		ID:            appID,
		Present:       true,
		Repo:          repo,
		Branch:        "main",
		DeployMode:    mode,
		Zone:          zone,
		PageSubdomain: pageSub,
	}
	if err := st.SetDesired(d); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	mgr.Enqueue(appID, reconcile.Options{})
	if s.Store != nil {
		_ = s.Store.WriteAudit(r.Context(), user, "install_start", appID+": "+repo)
	}
	writeJSON(w, map[string]any{"app_name": appID, "status": "queued"})
}

func repoBaseName(repo string) string {
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		return repo[i+1:]
	}
	return repo
}
