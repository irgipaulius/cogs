package gitpublic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/appcfg"
)

// PreviewApp is parsed cogs.app.toml fields for the install UI.
type PreviewApp struct {
	Name          string `json:"name"`
	Kind          string `json:"kind,omitempty"`
	DeployMode    string `json:"deploy_mode,omitempty"`
	Zone          string `json:"zone,omitempty"`
	PageSubdomain string `json:"page_subdomain,omitempty"`
	APISubdomain  string `json:"api_subdomain,omitempty"`
	PageHost      string `json:"page_host,omitempty"`
	APIHost       string `json:"api_host,omitempty"`
}

// RepoPreview is the result of a remote repo inspect (no clone).
type RepoPreview struct {
	FullName      string      `json:"full_name"`
	State         string      `json:"state"` // ready, need_gh, not_found, no_config, error
	Message       string      `json:"message,omitempty"`
	Private       bool        `json:"private"`
	DefaultBranch string      `json:"default_branch,omitempty"`
	ConfigPath    string      `json:"config_path,omitempty"`
	App           *PreviewApp `json:"app,omitempty"`
}

const (
	StateReady   = "ready"
	StateNeedGH  = "need_gh"
	StateNotFound = "not_found"
	StateNoConfig = "no_config"
	StateError   = "error"
)

// PreviewRemote inspects a public GitHub repo over HTTPS APIs.
func PreviewRemote(ctx context.Context, fullName string) (RepoPreview, error) {
	fullName = NormalizeFullName(fullName)
	if fullName == "" {
		return RepoPreview{}, fmt.Errorf("want owner/name")
	}
	meta, err := LookupUnauthenticated(ctx, fullName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "private") {
			return RepoPreview{
				FullName: fullName,
				State:    StateNeedGH,
				Message:  "Repository is private or does not exist. On the server run: gh auth login — then refresh this page.",
			}, nil
		}
		return RepoPreview{FullName: fullName, State: StateError, Message: err.Error()}, nil
	}
	return fillPreviewFromPublic(ctx, fullName, meta.DefaultBranch)
}

func fillPreviewFromPublic(ctx context.Context, fullName, branch string) (RepoPreview, error) {
	path, content, err := findAndFetchTomlPublic(ctx, fullName, branch)
	if err != nil {
		return RepoPreview{
			FullName:      fullName,
			State:         StateNoConfig,
			Message:       fmt.Sprintf("No cogs.app.toml found at repo root. Add one or use a monorepo path under apps/. (%v)", err),
			DefaultBranch: branch,
		}, nil
	}
	app, err := appcfg.Parse(content)
	if err != nil {
		return RepoPreview{
			FullName: fullName,
			State:    StateError,
			Message:  "Found cogs.app.toml but could not parse: " + err.Error(),
			ConfigPath: path,
		}, nil
	}
	pa := previewFromApp(app)
	return RepoPreview{
		FullName:      fullName,
		State:         StateReady,
		DefaultBranch: branch,
		ConfigPath:    path,
		App:           &pa,
		Message:       fmt.Sprintf("Ready — %s (%s)", app.Name, path),
	}, nil
}

func previewFromApp(app appcfg.App) PreviewApp {
	pa := PreviewApp{
		Name:          app.Name,
		Kind:          app.Kind,
		DeployMode:    app.DeployMode,
		Zone:          app.Zone,
		PageSubdomain: app.PageSubdomain,
		APISubdomain:  app.APISubdomain,
		PageHost:      app.PageHost(),
		APIHost:       app.APIHost(),
	}
	return pa
}

// PreviewFromToml builds preview from raw toml (gh-authenticated fetch).
func PreviewFromToml(fullName, configPath, toml string) (RepoPreview, error) {
	app, err := appcfg.Parse(toml)
	if err != nil {
		return RepoPreview{
			FullName:   fullName,
			State:      StateError,
			Message:    err.Error(),
			ConfigPath: configPath,
		}, nil
	}
	pa := previewFromApp(app)
	return RepoPreview{
		FullName:   fullName,
		State:      StateReady,
		ConfigPath: configPath,
		App:        &pa,
		Message:    fmt.Sprintf("Ready — %s (%s)", app.Name, configPath),
	}, nil
}

func findAndFetchTomlPublic(ctx context.Context, fullName, branch string) (path, content string, err error) {
	if branch == "" {
		branch = "main"
	}
	if content, err = fetchRawFile(ctx, fullName, appcfg.Filename, branch); err == nil {
		return appcfg.Filename, content, nil
	}
	searchPath, err := searchConfigPath(ctx, fullName)
	if err != nil || searchPath == "" {
		return "", "", fmt.Errorf("no cogs.app.toml")
	}
	content, err = fetchRawFile(ctx, fullName, searchPath, branch)
	if err != nil {
		return "", "", err
	}
	return searchPath, content, nil
}

func searchConfigPath(ctx context.Context, fullName string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	q := url.QueryEscape(fmt.Sprintf("filename:cogs.app.toml repo:%s", fullName))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/search/code?q="+q+"&per_page=5", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cogs-admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 403 {
		return "", fmt.Errorf("search rate limit")
	}
	var raw struct {
		Items []struct {
			Path string `json:"path"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", err
	}
	for _, it := range raw.Items {
		if strings.HasSuffix(it.Path, appcfg.Filename) {
			return it.Path, nil
		}
	}
	return "", fmt.Errorf("not found")
}

func fetchRawFile(ctx context.Context, fullName, path, ref string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	u := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s?ref=%s",
		fullName, url.PathEscape(path), url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "cogs-admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("404")
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("github %d", resp.StatusCode)
	}
	var raw struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", err
	}
	if raw.Encoding != "base64" {
		return "", fmt.Errorf("unexpected encoding")
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(raw.Content, "\n", ""))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
