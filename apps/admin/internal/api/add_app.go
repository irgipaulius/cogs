package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/appadd"
	"github.com/irgipaulius/cogs/pkg/appinstall"
	"github.com/irgipaulius/cogs/pkg/hostenv"
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
		ScaffoldOnly  bool   `json:"scaffold_only"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "bad json")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	var detail, appName string
	var err error
	if body.ScaffoldOnly {
		detail, err = appadd.Add(appadd.Options{
			WSRoot:        s.WorkspaceRoot,
			Name:          body.Name,
			GitRepo:       body.Repo,
			DeployMode:    body.DeployMode,
			PageSubdomain: body.PageSubdomain,
		})
		appName = body.Name
	} else {
		appName, _, detail, err = appinstall.Install(ctx, appinstall.Options{
			WSRoot:        s.WorkspaceRoot,
			RepoFull:      body.Repo,
			AppName:       body.Name,
			DeployMode:    body.DeployMode,
			PageSubdomain: body.PageSubdomain,
		})
	}
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.Store.WriteAudit(r.Context(), user, "add_app", detail)
	go runLaunchAfterAdd(s.WorkspaceRoot, appName)
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
	cmd.Env = append(hostenv.CLIEnv(wsRoot), "COGS_WORKSPACE_ROOT="+wsRoot)
	_, _ = cmd.CombinedOutput()
}
