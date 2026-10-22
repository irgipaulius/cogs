// Package ghcli wraps the GitHub CLI (gh) for host-level auth and repo listing.
package ghcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/irgipaulius/cogs/pkg/hostenv"
)

// AuthStatus is the current gh login state on this host.
type AuthStatus struct {
	Installed bool   `json:"installed"`
	LoggedIn  bool   `json:"logged_in"`
	Login     string `json:"login,omitempty"`
	Host      string `json:"host,omitempty"`
}

// Repo is one repository visible to the gh login.
type Repo struct {
	ID            string `json:"id"`
	FullName      string `json:"full_name"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HasCogsConfig bool   `json:"has_cogs_config"`
	CogsHint      string `json:"cogs_hint,omitempty"`
}

func ghBin() (string, error) {
	return exec.LookPath("gh")
}

func run(ctx context.Context, args ...string) (string, error) {
	bin, err := ghBin()
	if err != nil {
		return "", fmt.Errorf("gh not installed — install GitHub CLI (https://cli.github.com/)")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = hostenv.CLIEnv(os.Getenv("COGS_WORKSPACE_ROOT"))
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return out.String(), nil
}

// Status returns whether gh is installed and authenticated.
func Status(ctx context.Context) (AuthStatus, error) {
	st := AuthStatus{}
	if _, err := ghBin(); err != nil {
		return st, nil
	}
	st.Installed = true
	raw, err := run(ctx, "auth", "status", "--hostname", "github.com")
	if err != nil {
		return st, nil
	}
	text := raw
	if strings.Contains(text, "Logged in to") {
		st.LoggedIn = true
		st.Host = "github.com"
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "account ") {
			st.Login = strings.TrimSpace(strings.TrimPrefix(line, "account "))
			break
		}
	}
	if st.LoggedIn && st.Login == "" {
		if login, err := run(ctx, "api", "user", "-q", ".login"); err == nil {
			st.Login = strings.TrimSpace(login)
		}
	}
	return st, nil
}

// Token returns a bearer token from gh credential store.
func Token(ctx context.Context) (string, error) {
	st, err := Status(ctx)
	if err != nil {
		return "", err
	}
	if !st.LoggedIn {
		return "", fmt.Errorf("GitHub not logged in — run: gh auth login")
	}
	tok, err := run(ctx, "auth", "token")
	if err != nil {
		return "", err
	}
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return "", fmt.Errorf("gh auth token empty")
	}
	return tok, nil
}

// SetupGit configures git to use gh credentials.
func SetupGit(ctx context.Context) error {
	_, err := run(ctx, "auth", "setup-git")
	return err
}

type ghRepoRow struct {
	ID               string `json:"id"`
	NameWithOwner    string `json:"nameWithOwner"`
	IsPrivate        bool   `json:"isPrivate"`
	DefaultBranchRef *struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
}

// ListRepos returns repositories for the authenticated user (includes orgs gh can see).
func ListRepos(ctx context.Context) ([]Repo, error) {
	st, err := Status(ctx)
	if err != nil {
		return nil, err
	}
	if !st.LoggedIn {
		return nil, fmt.Errorf("GitHub not logged in — run: gh auth login")
	}
	raw, err := run(ctx, "repo", "list", "--limit", "1000", "--json", "id,nameWithOwner,isPrivate,defaultBranchRef")
	if err != nil {
		return nil, err
	}
	var rows []ghRepoRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	out := make([]Repo, 0, len(rows))
	for _, r := range rows {
		parts := strings.SplitN(r.NameWithOwner, "/", 2)
		owner, name := "", ""
		if len(parts) == 2 {
			owner, name = parts[0], parts[1]
		}
		branch := "main"
		if r.DefaultBranchRef != nil && r.DefaultBranchRef.Name != "" {
			branch = r.DefaultBranchRef.Name
		}
		repo := Repo{
			ID:            r.ID,
			FullName:      r.NameWithOwner,
			Owner:         owner,
			Name:          name,
			Private:       r.IsPrivate,
			DefaultBranch: branch,
		}
		hint, ok := detectCogsConfig(ctx, r.NameWithOwner)
		repo.HasCogsConfig = ok
		repo.CogsHint = hint
		out = append(out, repo)
	}
	return out, nil
}

func detectCogsConfig(ctx context.Context, fullName string) (hint string, ok bool) {
	paths := []string{"cogs.app.toml", "cogs.workspace.json", "cogs.launch.json"}
	for _, p := range paths {
		if fileExists(ctx, fullName, p) {
			return p, true
		}
	}
	// Monorepo: apps/<any>/cogs.app.toml — sample via search API (best effort).
	raw, err := run(ctx, "api", fmt.Sprintf("search/code?q=filename:cogs.app.toml+repo:%s", fullName), "-q", ".items[0].path")
	if err == nil && strings.TrimSpace(raw) != "" && raw != "null" {
		return strings.TrimSpace(raw), true
	}
	return "", false
}

func fileExists(ctx context.Context, fullName, path string) bool {
	_, err := run(ctx, "api", fmt.Sprintf("repos/%s/contents/%s", fullName, path), "-q", ".name")
	return err == nil
}

// LoginDevice runs gh auth login (device flow, no browser). Blocks until complete or ctx cancel.
func LoginDevice(ctx context.Context) error {
	_, err := run(ctx, "auth", "login", "--hostname", "github.com", "--git-protocol", "https",
		"--skip-ssh-key", "--web=false", "-s", "repo,read:org")
	if err != nil {
		return err
	}
	return SetupGit(ctx)
}

// StartLoginAsync runs device login in the background; lines append to logPath.
func StartLoginAsync(logPath string) error {
	bin, err := ghBin()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "auth", "login", "--hostname", "github.com", "--git-protocol", "https",
		"--skip-ssh-key", "--web=false", "-s", "repo,read:org")
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Start(); err != nil {
		f.Close()
		return err
	}
	go func() {
		_ = cmd.Wait()
		_ = SetupGit(context.Background())
		_ = f.Close()
	}()
	return nil
}

// ParseDeviceCodeFromLog extracts one-time code from gh login output.
func ParseDeviceCodeFromLog(text string) (code, verifyURL string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "one-time code:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				code = strings.TrimSpace(parts[len(parts)-1])
			}
		}
		if strings.HasPrefix(line, "Open this URL") || strings.Contains(line, "https://github.com/login/device") {
			for _, w := range strings.Fields(line) {
				if strings.HasPrefix(w, "http") {
					verifyURL = strings.Trim(w, " ")
				}
			}
		}
	}
	if verifyURL == "" {
		verifyURL = "https://github.com/login/device"
	}
	return code, verifyURL
}

// RepoDatabaseID returns the numeric GitHub repository id (gh repo list's
// "id" is a GraphQL node id; Cloudflare Workers Builds needs the number).
func RepoDatabaseID(ctx context.Context, fullName string) (int64, error) {
	raw, err := run(ctx, "api", "repos/"+fullName, "--jq", ".id")
	if err != nil {
		return 0, err
	}
	var id int64
	if _, err := fmt.Sscan(strings.TrimSpace(raw), &id); err != nil || id == 0 {
		return 0, fmt.Errorf("repo id for %s: unexpected %q", fullName, strings.TrimSpace(raw))
	}
	return id, nil
}
