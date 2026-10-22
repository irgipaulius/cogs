package api

import (
	"encoding/json"
	"net/http"
	"os/exec"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/appadd"
	"github.com/irgipaulius/cogs/pkg/githubauth"
)

func (s *Server) handleAddApp(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	if s.WorkspaceRoot == "" {
		writeAPIError(w, http.StatusServiceUnavailable, "workspace root not configured on admin server")
		return
	}
	var body struct {
		Name          string `json:"name"`
		Repo          string `json:"repo"`
		DeployMode    string `json:"deploy_mode"`
		PageSubdomain string `json:"page_subdomain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "bad json")
		return
	}
	c, _ := githubauth.Load(s.WorkspaceRoot)
	if c.Token == "" {
		writeAPIError(w, http.StatusBadRequest, "GitHub not signed in — connect GitHub before adding an app")
		return
	}
	detail, err := appadd.Add(appadd.Options{
		WSRoot:        s.WorkspaceRoot,
		Name:          body.Name,
		GitRepo:       body.Repo,
		DeployMode:    body.DeployMode,
		PageSubdomain: body.PageSubdomain,
	})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.Store.WriteAudit(r.Context(), user, "add_app", detail)
	wsRoot := s.WorkspaceRoot
	name := body.Name
	go runLaunchAfterAdd(wsRoot, name)
	writeJSON(w, map[string]string{
		"detail": detail,
		"launch": "started in background (cogs launch -y --no-serve)",
	})
}

func runLaunchAfterAdd(wsRoot, name string) {
	appDir, err := appadd.AppDir(wsRoot, name)
	if err != nil {
		return
	}
	bin, err := exec.LookPath("cogs")
	if err != nil {
		return
	}
	cmd := exec.Command(bin, "launch", appDir, "-y", "--no-serve")
	cmd.Dir = wsRoot
	_, _ = cmd.CombinedOutput()
}
