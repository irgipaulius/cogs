package ghcli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/gitpublic"
)

// PreviewRepo inspects a repo using gh (works for private repos).
func PreviewRepo(ctx context.Context, fullName string) (gitpublic.RepoPreview, error) {
	fullName = gitpublic.NormalizeFullName(fullName)
	if fullName == "" {
		return gitpublic.RepoPreview{}, fmt.Errorf("want owner/name")
	}
	st, err := Status(ctx)
	if err != nil || !st.LoggedIn {
		return gitpublic.RepoPreview{}, fmt.Errorf("GitHub not logged in")
	}
	raw, err := run(ctx, "api", "repos/"+fullName)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return gitpublic.RepoPreview{
				FullName: fullName,
				State:    gitpublic.StateNotFound,
				Message:  "Repository not found (check owner/name).",
			}, nil
		}
		return gitpublic.RepoPreview{}, err
	}
	var meta struct {
		FullName      string `json:"full_name"`
		Private       bool   `json:"private"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return gitpublic.RepoPreview{}, err
	}
	branch := meta.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	path, toml, err := findTomlViaGH(ctx, fullName, branch)
	if err != nil {
		msg := "No cogs.app.toml in this repository."
		if meta.Private {
			msg = "No cogs.app.toml found. Add one at the repo root or under apps/<name>/."
		}
		return gitpublic.RepoPreview{
			FullName:      meta.FullName,
			State:         gitpublic.StateNoConfig,
			Message:       msg,
			Private:       meta.Private,
			DefaultBranch: branch,
		}, nil
	}
	prev, err := gitpublic.PreviewFromToml(meta.FullName, path, toml)
	if err != nil {
		return prev, err
	}
	prev.Private = meta.Private
	prev.DefaultBranch = branch
	return prev, nil
}

func findTomlViaGH(ctx context.Context, fullName, branch string) (path, content string, err error) {
	if content, err = ghFetchFile(ctx, fullName, appcfg.Filename, branch); err == nil {
		return appcfg.Filename, content, nil
	}
	raw, err := run(ctx, "api", fmt.Sprintf("search/code?q=filename:cogs.app.toml+repo:%s", fullName))
	if err != nil {
		return "", "", err
	}
	var search struct {
		Items []struct {
			Path string `json:"path"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(raw), &search); err != nil || len(search.Items) == 0 {
		return "", "", fmt.Errorf("no config")
	}
	for _, it := range search.Items {
		p := it.Path
		if !strings.HasSuffix(p, appcfg.Filename) {
			continue
		}
		if content, err = ghFetchFile(ctx, fullName, p, branch); err == nil {
			return p, content, nil
		}
	}
	return "", "", fmt.Errorf("no config")
}

func ghFetchFile(ctx context.Context, fullName, path, ref string) (string, error) {
	raw, err := run(ctx, "api", fmt.Sprintf("repos/%s/contents/%s?ref=%s", fullName, path, ref))
	if err != nil {
		return "", err
	}
	var file struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal([]byte(raw), &file); err != nil {
		return "", err
	}
	if file.Encoding != "base64" {
		return "", fmt.Errorf("encoding")
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
