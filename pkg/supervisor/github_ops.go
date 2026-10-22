package supervisor

import (
	"context"
	"encoding/json"
	"time"

	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/githubauth"
)

func githubStatus(wsRoot string) json.RawMessage {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	gh, _ := ghcli.Status(ctx)
	if gh.LoggedIn {
		b, _ := json.Marshal(map[string]any{
			"signed_in": true,
			"login":     gh.Login,
			"source":    "gh",
		})
		return b
	}
	c, _ := githubauth.Load(wsRoot)
	b, _ := json.Marshal(map[string]any{
		"signed_in": c.Token != "",
		"login":     c.Login,
		"source":    "legacy",
	})
	return b
}

func githubDeviceStart(wsRoot string) (json.RawMessage, error) {
	_ = wsRoot
	return nil, errMissingGitHub
}

func githubDevicePoll(wsRoot, deviceCode string) (json.RawMessage, error) {
	_ = wsRoot
	_ = deviceCode
	return nil, errMissingGitHub
}

func githubRepos(wsRoot string) (json.RawMessage, error) {
	_ = wsRoot
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	repos, err := ghcli.ListRepos(ctx)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(repos)
	return b, err
}

var errMissingGitHub = &githubErr{msg: "GitHub not logged in on host — run: gh auth login"}

type githubErr struct{ msg string }

func (e *githubErr) Error() string { return e.msg }
