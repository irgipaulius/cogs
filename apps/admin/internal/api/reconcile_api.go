package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/reconcile"
	"github.com/irgipaulius/cogs/pkg/state"
)

func (s *Server) ensureState() (*state.Store, *reconcile.Manager, error) {
	if s.WorkspaceRoot == "" {
		return nil, nil, fmt.Errorf("workspace root not configured")
	}
	if s.StateDB == nil {
		st, err := state.Open(s.WorkspaceRoot)
		if err != nil {
			return nil, nil, err
		}
		s.StateDB = st
		s.Reconciler = reconcile.NewManager(st)
	}
	return s.StateDB, s.Reconciler, nil
}

func (s *Server) handleReconcileStream(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	st, _, err := s.ensureState()
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	write := func() {
		b, _ := st.ExportAppsJSON()
		fmt.Fprintf(w, "event: apps\ndata: %s\n\n", b)
		fl.Flush()
	}
	write()
	ch := st.Subscribe()
	defer st.Unsubscribe(ch)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			write()
		case <-tick.C:
			write()
		}
	}
}

func (s *Server) handleAppReconcile(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	_, mgr, err := s.ensureState()
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	mgr.Enqueue(name, reconcile.Options{})
	writeJSON(w, map[string]string{"status": "queued", "app": name})
}

func (s *Server) handleAppRemoveReconcile(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	st, mgr, err := s.ensureState()
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	app, err := st.GetApp(name)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "app not found")
		return
	}
	d := app.AppDesired
	d.Present = false
	if err := st.SetDesired(d); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	mgr.Enqueue(name, reconcile.Options{})
	writeJSON(w, map[string]string{"status": "remove queued"})
}

func (s *Server) handleReconcileLog(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	name := r.PathValue("name")
	st, _, err := s.ensureState()
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	app, err := st.GetApp(name)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "not found")
		return
	}
	if app.LogPath == "" {
		writeAPIError(w, http.StatusNotFound, "no log")
		return
	}
	p := app.LogPath
	if !filepath.IsAbs(p) {
		p = filepath.Join(s.WorkspaceRoot, app.LogPath)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]string{"log": string(b)})
}
