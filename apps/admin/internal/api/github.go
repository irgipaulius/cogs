package api

import (
	"encoding/json"
	"net/http"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/githubauth"
)

func (s *Server) githubStatusJSON() json.RawMessage {
	c, _ := githubauth.Load(s.WorkspaceRoot)
	b, _ := json.Marshal(map[string]any{
		"signed_in": c.Token != "",
		"login":     c.Login,
	})
	return b
}

func (s *Server) handleGitHubStatus(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	if s.WorkspaceRoot == "" {
		writeAPIError(w, http.StatusServiceUnavailable, "workspace root not configured on admin server")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(s.githubStatusJSON())
}

func (s *Server) handleGitHubLoginStart(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	clientID, err := githubauth.ClientID()
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	start, err := githubauth.StartDeviceFlow(clientID)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"user_code":        start.UserCode,
		"verification_uri": start.VerificationURI,
		"device_code":      start.DeviceCode,
		"interval":         start.Interval,
	})
}

func (s *Server) handleGitHubLoginPoll(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	var body struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "bad json")
		return
	}
	clientID, err := githubauth.ClientID()
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := githubauth.PollDeviceFlowOnce(clientID, body.DeviceCode)
	if err != nil {
		if err.Error() == "authorization_pending" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(s.githubStatusJSON())
			return
		}
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := githubauth.Save(s.WorkspaceRoot, c); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(s.githubStatusJSON())
}

func (s *Server) handleGitHubRepos(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	c, err := githubauth.Load(s.WorkspaceRoot)
	if err != nil || c.Token == "" {
		writeAPIError(w, http.StatusBadRequest, "GitHub not signed in — use Sign in with GitHub")
		return
	}
	repos, err := githubauth.ListRepos(c.Token)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, repos)
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
