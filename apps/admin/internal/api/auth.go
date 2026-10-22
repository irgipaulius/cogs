package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/wranglercli"
)

const reposCacheTTL = 5 * time.Minute

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	gh, _ := ghcli.Status(ctx)
	cf, _ := wranglercli.Status(ctx)
	writeJSON(w, map[string]any{
		"github":     gh,
		"cloudflare": cf,
	})
}

func (s *Server) handleRepos(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	gh, err := ghcli.Status(ctx)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}
	if !gh.LoggedIn {
		writeAPIError(w, http.StatusBadRequest, "GitHub not logged in on this host — SSH to the server and run: gh auth login && gh auth setup-git")
		return
	}
	repos, err := s.cachedRepos(ctx)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q != "" {
		filtered := repos[:0]
		for _, repo := range repos {
			if strings.Contains(strings.ToLower(repo.FullName), q) {
				filtered = append(filtered, repo)
			}
		}
		repos = filtered
	}
	writeJSON(w, repos)
}

func (s *Server) cachedRepos(ctx context.Context) ([]ghcli.Repo, error) {
	s.reposMu.Lock()
	defer s.reposMu.Unlock()
	if len(s.reposCache) > 0 && time.Since(s.reposAt) < reposCacheTTL {
		out := make([]ghcli.Repo, len(s.reposCache))
		copy(out, s.reposCache)
		return out, nil
	}
	repos, err := ghcli.ListRepos(ctx)
	if err != nil {
		return nil, err
	}
	s.reposCache = repos
	s.reposAt = time.Now()
	out := make([]ghcli.Repo, len(repos))
	copy(out, repos)
	return out, nil
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
