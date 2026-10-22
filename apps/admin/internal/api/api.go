package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/installjob"
	"github.com/irgipaulius/cogs/pkg/supervisor"
)

const sessionCookie = "cogs_admin_session"

// Server is the admin HTTP API + SPA shell.
type Server struct {
	Store         *adminstore.Store
	Supervisor    *supervisor.Client
	WorkspaceRoot string
	SecureCookie  bool

	reposMu    sync.Mutex
	reposCache []ghcli.Repo
	reposAt    time.Time

	Installs *installjob.Manager
}

func (s *Server) Handler(mux *http.ServeMux, spa http.Handler) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.auth(s.handleMe))
	mux.HandleFunc("GET /api/metrics/latest", s.auth(s.handleMetricsLatest))
	mux.HandleFunc("GET /api/metrics/stream", s.auth(s.handleMetricsStream))
	mux.HandleFunc("GET /api/cockpit", s.auth(s.handleCockpit))
	mux.HandleFunc("GET /api/apps/{name}/logs", s.auth(s.handleAppLogs))
	mux.HandleFunc("GET /api/apps/{name}/telemetry", s.auth(s.handleAppTelemetry))
	mux.HandleFunc("GET /api/registry", s.auth(s.handleRegistry))
	mux.HandleFunc("GET /api/sql/tables", s.auth(s.handleSQLTables))
	mux.HandleFunc("POST /api/sql", s.auth(s.handleSQL))
	mux.HandleFunc("POST /api/redeploy", s.auth(s.handleRedeploy))
	mux.HandleFunc("POST /api/rollback", s.auth(s.handleRollback))
	mux.HandleFunc("GET /api/auth/status", s.auth(s.handleAuthStatus))
	mux.HandleFunc("GET /api/repos", s.auth(s.handleRepos))
	mux.HandleFunc("GET /api/repos/preview", s.auth(s.handleRepoPreview))
	mux.HandleFunc("POST /api/apps", s.auth(s.handleAddApp))
	mux.HandleFunc("GET /api/installs", s.auth(s.handleListInstalls))
	mux.HandleFunc("GET /api/installs/stream", s.auth(s.handleInstallStream))
	mux.HandleFunc("GET /api/installs/{id}", s.auth(s.handleGetInstall))
	mux.HandleFunc("GET /api/installs/{id}/log", s.auth(s.handleInstallLog))
	mux.HandleFunc("POST /api/installs/{id}/retry", s.auth(s.handleInstallRetry))
	mux.HandleFunc("GET /api/hostnames/check", s.auth(s.handleHostnameCheck))
	mux.HandleFunc("GET /api/users", s.auth(s.handleListUsers))
	mux.HandleFunc("POST /api/users", s.auth(s.handleCreateUser))
	mux.HandleFunc("PUT /api/users/{id}/grants", s.auth(s.handleSetGrants))
	mux.HandleFunc("GET /api/audit", s.auth(s.handleAudit))
	mux.Handle("/", spa)
}

type authed func(http.ResponseWriter, *http.Request, *adminstore.EffectiveUser)

func (s *Server) auth(next authed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid := sessionFromRequest(r)
		user, err := s.Store.ResolveSession(r.Context(), sid)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r, user)
	}
}

func sessionFromRequest(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return r.Header.Get("X-Cogs-Admin-Session")
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	sid, err := s.Store.Authenticate(r.Context(), body.Username, body.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.SecureCookie,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sid := sessionFromRequest(r)
	_ = s.Store.Logout(r.Context(), sid)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, MaxAge: -1, Path: "/"})
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	writeJSON(w, map[string]any{
		"username": user.Username,
		"is_root":  user.IsRoot,
		"grants":   user.Grants,
	})
}

func (s *Server) supCall(req supervisor.Request) (supervisor.Response, error) {
	return s.Supervisor.Call(req)
}

func (s *Server) handleMetricsLatest(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	resp, err := s.supCall(supervisor.Request{Op: "metrics", Session: sessionFromRequest(r)})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleMetricsStream(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	sid := sessionFromRequest(r)
	resp, err := s.supCall(supervisor.Request{Op: "cockpit", Session: sid})
	if err == nil && resp.OK {
		fmt.Fprintf(w, "event: history\ndata: %s\n\n", string(resp.Data))
		fl.Flush()
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			resp, err := s.supCall(supervisor.Request{Op: "metrics_stream_hint", Session: sid})
			if err != nil || !resp.OK {
				continue
			}
			fmt.Fprintf(w, "event: tick\ndata: %s\n\n", string(resp.Data))
			fl.Flush()
		}
	}
}

func (s *Server) handleCockpit(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	resp, err := s.supCall(supervisor.Request{Op: "cockpit", Session: sessionFromRequest(r)})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleAppLogs(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	app := r.PathValue("name")
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "process"
	}
	resp, err := s.supCall(supervisor.Request{
		Op: "logs", Session: sessionFromRequest(r), App: app, LogKind: kind,
	})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleAppTelemetry(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	app := r.PathValue("name")
	resp, err := s.supCall(supervisor.Request{
		Op: "telemetry", Session: sessionFromRequest(r), App: app,
	})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleRegistry(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	resp, err := s.supCall(supervisor.Request{Op: "registry", Session: sessionFromRequest(r)})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleSQLTables(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	dbID := r.URL.Query().Get("db_id")
	app := r.URL.Query().Get("app")
	resp, err := s.supCall(supervisor.Request{
		Op: "sql_tables", Session: sessionFromRequest(r), DBID: dbID, App: app,
	})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleSQL(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	var body struct {
		DBID  string `json:"db_id"`
		App   string `json:"app"`
		SQL   string `json:"sql"`
		Write bool   `json:"write"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	resp, err := s.supCall(supervisor.Request{
		Op: "sql", Session: sessionFromRequest(r),
		DBID: body.DBID, App: body.App, SQL: body.SQL, Write: body.Write,
	})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	var body struct {
		App string `json:"app"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	resp, err := s.supCall(supervisor.Request{
		Op: "rollback", Session: sessionFromRequest(r), App: body.App,
	})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	_ = s.Store.WriteAudit(r.Context(), user, "rollback", body.App)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleRedeploy(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	var body struct {
		App string `json:"app"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	resp, err := s.supCall(supervisor.Request{
		Op: "redeploy", Session: sessionFromRequest(r), App: body.App,
	})
	if err != nil || !resp.OK {
		http.Error(w, errMsg(resp, err), http.StatusBadGateway)
		return
	}
	_ = s.Store.WriteAudit(r.Context(), user, "redeploy", body.App)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp.Data)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot && !user.HasPermission("*", adminstore.PermUsersManage) {
		http.Error(w, "denied", http.StatusForbidden)
		return
	}
	users, err := s.Store.ListUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type row struct {
		ID       int64            `json:"id"`
		Username string           `json:"username"`
		IsRoot   bool             `json:"is_root"`
		Grants   []adminstore.Grant `json:"grants,omitempty"`
	}
	var out []row
	for _, u := range users {
		grants, _ := s.Store.ListGrantsForUser(r.Context(), u.ID)
		out = append(out, row{ID: u.ID, Username: u.Username, IsRoot: u.IsRoot, Grants: grants})
	}
	writeJSON(w, out)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot && !user.HasPermission("*", adminstore.PermUsersManage) {
		http.Error(w, "denied", http.StatusForbidden)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	id, err := s.Store.CreateUser(r.Context(), body.Username, body.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"id": id})
}

func (s *Server) handleSetGrants(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot && !user.HasPermission("*", adminstore.PermUsersManage) {
		http.Error(w, "denied", http.StatusForbidden)
		return
	}
	var uid int64
	_, _ = fmt.Sscan(r.PathValue("id"), &uid)
	var body struct {
		Grants []adminstore.Grant `json:"grants"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	for _, g := range body.Grants {
		if !user.IsRoot && !user.CanGrant(g.Permission) {
			http.Error(w, "cannot grant "+g.Permission, http.StatusForbidden)
			return
		}
	}
	if err := s.Store.SetGrants(r.Context(), uid, body.Grants); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		http.Error(w, "denied", http.StatusForbidden)
		return
	}
	rows, err := s.Store.ListAudit(r.Context(), 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, rows)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func errMsg(resp supervisor.Response, err error) string {
	if err != nil {
		return err.Error()
	}
	return resp.Error
}

// PingSupervisor checks socket connectivity.
func PingSupervisor(ctx context.Context, c *supervisor.Client, session string) error {
	resp, err := c.Call(supervisor.Request{Op: "ping", Session: session})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func IsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
