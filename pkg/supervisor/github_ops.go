package supervisor

import (
	"encoding/json"

	"github.com/irgipaulius/cogs/pkg/githubauth"
)

func githubStatus(wsRoot string) json.RawMessage {
	c, _ := githubauth.Load(wsRoot)
	b, _ := json.Marshal(map[string]any{
		"signed_in": c.Token != "",
		"login":     c.Login,
	})
	return b
}

func githubDeviceStart(wsRoot string) (json.RawMessage, error) {
	clientID, err := githubauth.ClientID()
	if err != nil {
		return nil, err
	}
	start, err := githubauth.StartDeviceFlow(clientID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"user_code":        start.UserCode,
		"verification_uri": start.VerificationURI,
		"device_code":      start.DeviceCode,
		"interval":         start.Interval,
	})
}

func githubDevicePoll(wsRoot, deviceCode string) (json.RawMessage, error) {
	clientID, err := githubauth.ClientID()
	if err != nil {
		return nil, err
	}
	c, err := githubauth.PollDeviceFlowOnce(clientID, deviceCode)
	if err != nil {
		return nil, err
	}
	if err := githubauth.Save(wsRoot, c); err != nil {
		return nil, err
	}
	return githubStatus(wsRoot), nil
}

func githubRepos(wsRoot string) (json.RawMessage, error) {
	c, err := githubauth.Load(wsRoot)
	if err != nil || c.Token == "" {
		return nil, errMissingGitHub
	}
	repos, err := githubauth.ListRepos(c.Token)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(repos)
	return b, err
}

var errMissingGitHub = &githubErr{msg: "GitHub not signed in — use Login with GitHub"}

type githubErr struct{ msg string }

func (e *githubErr) Error() string { return e.msg }
