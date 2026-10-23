package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"os"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/gitpublic"
	"github.com/irgipaulius/cogs/pkg/hostsetup"
	"github.com/irgipaulius/cogs/pkg/cfclient"
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
	if s.WorkspaceRoot != "" {
		_ = os.Setenv("COGS_WORKSPACE_ROOT", s.WorkspaceRoot)
	}
	gh, _ := ghcli.Status(ctx)
	cf, _ := wranglercli.Status(ctx)
	cfAuth := cfclient.CloudflareAuthStatus(ctx, s.WorkspaceRoot)
	cfMap := map[string]any{
		"installed":    cf.Installed,
		"logged_in":    cf.LoggedIn,
		"email":        cf.Email,
		"account_id":   cf.AccountID,
		"account_name": cf.Account,
		"api_token_ok": cfAuth["api_token_ok"],
	}
	if msg, _ := cfAuth["message"].(string); msg != "" {
		cfMap["auth_message"] = msg
	}
	if id, _ := cfAuth["account_id"].(string); id != "" {
		cfMap["account_id"] = id
	}
	writeJSON(w, map[string]any{
		"github":     gh,
		"cloudflare": cfMap,
		"setup": map[string]any{
			"cogs_home":               os.Getenv("COGS_HOME"),
			"github_install_hint":     "cogs host setup  (or: pkg install gh / brew install gh)",
			"github_login_commands":   hostsetup.GitHubLoginHint(),
			"wrangler_login_command":  "wrangler login",
		},
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
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if !gh.LoggedIn {
		writeJSON(w, publicReposOnly(ctx, q))
		return
	}
	repos, err := s.cachedRepos(ctx)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}
	if q != "" {
		qLower := strings.ToLower(q)
		filtered := repos[:0]
		for _, repo := range repos {
			if strings.Contains(strings.ToLower(repo.FullName), qLower) {
				filtered = append(filtered, repo)
			}
		}
		repos = filtered
	}
	writeJSON(w, repos)
}

func publicReposOnly(ctx context.Context, q string) []ghcli.Repo {
	if full := gitpublic.NormalizeFullName(q); full != "" {
		pub, err := gitpublic.LookupUnauthenticated(ctx, full)
		if err != nil {
			return nil
		}
		return []ghcli.Repo{{
			FullName:      pub.FullName,
			Private:       false,
			DefaultBranch: pub.DefaultBranch,
			HasCogsConfig: pub.HasCogsConfig,
			CogsHint:      pub.CogsHint,
		}}
	}
	return nil
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
