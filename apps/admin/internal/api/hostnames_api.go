package api

import (
	"net/http"
	"strings"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/hostcheck"
)

func (s *Server) handleHostnameCheck(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	sub := strings.TrimSpace(r.URL.Query().Get("subdomain"))
	apiSub := strings.TrimSpace(r.URL.Query().Get("api_subdomain"))
	zone := strings.TrimSpace(r.URL.Query().Get("zone"))
	skip := strings.TrimSpace(r.URL.Query().Get("skip_app"))
	if err := hostcheck.ValidateSubdomain(sub); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error(), "hostname": hostcheck.PageHostname(sub, zone)})
		return
	}
	if apiSub != "" {
		if err := hostcheck.ValidateSubdomain(apiSub); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error(), "api_hostname": apiSub + "." + zone})
			return
		}
	}
	host := hostcheck.PageHostname(sub, zone)
	conflicts, err := hostcheck.FindHostConflicts(s.WorkspaceRoot, sub, zone, skip)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if st, _, err := s.ensureState(); err == nil && st != nil && zone != "" {
		if err := st.CheckHostnames(skip, sub, apiSub, zone); err != nil {
			conflicts = append(conflicts, err.Error())
		}
	}
	out := map[string]any{"ok": len(conflicts) == 0, "hostname": host}
	if apiSub != "" && zone != "" {
		out["api_hostname"] = apiSub + "." + zone
	}
	if len(conflicts) > 0 {
		out["error"] = conflicts[0]
	}
	writeJSON(w, out)
}
