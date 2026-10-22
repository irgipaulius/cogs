// Package gitpublic clones and inspects public GitHub repositories without gh auth.
package gitpublic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Repo is a minimal public repository descriptor for the admin picker.
type Repo struct {
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HasCogsConfig bool   `json:"has_cogs_config"`
	CogsHint      string `json:"cogs_hint,omitempty"`
}

// NormalizeFullName returns owner/repo or empty if invalid.
func NormalizeFullName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://github.com/")
	s = strings.TrimSuffix(s, ".git")
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

// AccessibleWithoutAuth reports whether git can read the repo over anonymous HTTPS.
func AccessibleWithoutAuth(ctx context.Context, fullName string) bool {
	fullName = NormalizeFullName(fullName)
	if fullName == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	url := "https://github.com/" + fullName + ".git"
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", url, "HEAD")
	cmd.Env = []string{"GIT_TERMINAL_PROMPT=0", "PATH=" + osPath()}
	return cmd.Run() == nil
}

func osPath() string {
	if p := os.Getenv("PATH"); p != "" {
		return p
	}
	return "/usr/bin:/bin:/usr/local/bin"
}

// LookupUnauthenticated returns metadata for a public repo (404 when private or missing).
func LookupUnauthenticated(ctx context.Context, fullName string) (Repo, error) {
	fullName = NormalizeFullName(fullName)
	if fullName == "" {
		return Repo{}, fmt.Errorf("want repo as owner/name")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+fullName, nil)
	if err != nil {
		return Repo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cogs-admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Repo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Repo{}, fmt.Errorf("repository not found or private — run gh auth login on the host for private repos")
	}
	if resp.StatusCode >= 300 {
		return Repo{}, fmt.Errorf("github api %d", resp.StatusCode)
	}
	var raw struct {
		FullName    string `json:"full_name"`
		Private     bool   `json:"private"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Repo{}, err
	}
	if raw.Private {
		return Repo{}, fmt.Errorf("repository is private — run gh auth login on the host")
	}
	branch := raw.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	repo := Repo{FullName: raw.FullName, Private: false, DefaultBranch: branch}
	if fileExistsPublic(ctx, fullName, "cogs.app.toml") {
		repo.HasCogsConfig = true
		repo.CogsHint = "cogs.app.toml"
	}
	return repo, nil
}

func fileExistsPublic(ctx context.Context, fullName, path string) bool {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", fullName, path)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cogs-admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// CloneHTTPS shallow-clones a public repository.
func CloneHTTPS(ctx context.Context, fullName, dest string) error {
	fullName = NormalizeFullName(fullName)
	if fullName == "" {
		return fmt.Errorf("invalid repo name")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	url := "https://github.com/" + fullName + ".git"
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, dest)
	cmd.Env = []string{"GIT_TERMINAL_PROMPT=0", "PATH=" + osPath()}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(strings.ToLower(msg), "authentication") || strings.Contains(msg, "403") {
			return fmt.Errorf("repository is private or requires auth — run: gh auth login")
		}
		return fmt.Errorf("git clone: %w: %s", err, msg)
	}
	return nil
}
