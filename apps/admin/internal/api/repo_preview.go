package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/gitpublic"
	"github.com/irgipaulius/cogs/pkg/wranglercli"
)

func (s *Server) handleRepoPreview(w http.ResponseWriter, r *http.Request, user *adminstore.EffectiveUser) {
	if !user.IsRoot {
		writeAPIError(w, http.StatusForbidden, "denied")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if gitpublic.NormalizeFullName(q) == "" {
		writeAPIError(w, http.StatusBadRequest, "q must be owner/repo")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	var preview gitpublic.RepoPreview
	var err error
	if st, _ := ghcli.Status(ctx); st.LoggedIn {
		preview, err = ghcli.PreviewRepo(ctx, q)
	} else {
		preview, err = gitpublic.PreviewRemote(ctx, q)
	}
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}

	zones, _ := wranglercli.ListZones(ctx)
	zoneNames := make([]string, 0, len(zones))
	for _, z := range zones {
		zoneNames = append(zoneNames, z.Name)
	}

	effectiveZone := ""
	if preview.App != nil {
		effectiveZone = preview.App.Zone
	}
	zoneOverride := strings.TrimSpace(r.URL.Query().Get("zone"))
	if zoneOverride != "" {
		effectiveZone = zoneOverride
	}
	hosts := map[string]string{}
	if preview.App != nil && effectiveZone != "" {
		a := appcfg.App{
			Name:          preview.App.Name,
			Kind:          preview.App.Kind,
			Zone:          effectiveZone,
			PageSubdomain: preview.App.PageSubdomain,
			APISubdomain:  preview.App.APISubdomain,
		}
		if ph := a.PageHost(); ph != "" {
			hosts["page"] = "https://" + ph
		}
		if ah := a.APIHost(); ah != "" {
			hosts["api"] = "https://" + ah
		}
	}

	writeJSON(w, map[string]any{
		"preview": preview,
		"zones":   zoneNames,
		"hosts":   hosts,
	})
}
