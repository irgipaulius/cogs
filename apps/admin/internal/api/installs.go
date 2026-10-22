package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/hostcheck"
	"github.com/irgipaulius/cogs/pkg/installjob"
)

func (s *Server) ensureInstalls() *installjob.Manager {
	if s.Installs == nil && s.WorkspaceRoot != "" {
		s.Installs = installjob.NewManager(s.WorkspaceRoot)
		s.Installs.ResumePending()
	}
	return s.Installs
}

func (s *Server) handleListInstalls(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	mgr := s.ensureInstalls()
	if mgr == nil {
		writeJSON(w, []any{})
		return
	}
	jobs, err := mgr.List()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, jobs)
}

func (s *Server) handleGetInstall(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	id := r.PathValue("id")
	mgr := s.ensureInstalls()
	job, err := mgr.Get(id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "install not found")
		return
	}
	writeJSON(w, job)
}

func (s *Server) handleInstallLog(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	id := r.PathValue("id")
	mgr := s.ensureInstalls()
	tail, err := mgr.LogTail(id, 512*1024)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]string{"log": tail})
}

func (s *Server) handleInstallRetry(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	id := r.PathValue("id")
	mgr := s.ensureInstalls()
	job, err := mgr.Retry(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, job)
}

func (s *Server) handleInstallStream(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	mgr := s.ensureInstalls()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	writeInstallSSE := func() {
		jobs, err := mgr.List()
		if err != nil {
			return
		}
		b, _ := json.Marshal(jobs)
		fmt.Fprintf(w, "event: installs\ndata: %s\n\n", b)
		fl.Flush()
	}
	writeInstallSSE()

	ch := mgr.Subscribe()
	defer mgr.Unsubscribe(ch)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			writeInstallSSE()
		case <-tick.C:
			writeInstallSSE()
		}
	}
}

func (s *Server) handleHostnameCheck(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	sub := strings.TrimSpace(r.URL.Query().Get("subdomain"))
	zone := strings.TrimSpace(r.URL.Query().Get("zone"))
	skip := strings.TrimSpace(r.URL.Query().Get("skip_app"))
	if err := hostcheck.ValidateSubdomain(sub); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error(), "hostname": hostcheck.PageHostname(sub, zone)})
		return
	}
	host := hostcheck.PageHostname(sub, zone)
	conflict, err := hostcheck.FindPageHostConflict(s.WorkspaceRoot, sub, zone, skip)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{"ok": conflict == "", "hostname": host}
	if conflict != "" {
		out["conflict_app"] = conflict
		out["error"] = fmt.Sprintf("hostname already used by app %q", conflict)
	}
	writeJSON(w, out)
}
