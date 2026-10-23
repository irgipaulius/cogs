package supervisor

import (
	"time"

	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/state"
)

// ProvisionUI mirrors legacy provision.State for the admin dashboard.
type ProvisionUI struct {
	Ready   bool     `json:"ready"`
	Status  string   `json:"status"`
	Reasons []string `json:"reasons,omitempty"`
}

// CockpitBoard is the full admin dashboard payload.
type CockpitBoard struct {
	At           time.Time                   `json:"at"`
	History      []MetricsSample             `json:"history"`
	Latest       MetricsSample               `json:"latest"`
	Telemetry    map[string]cockpit.Snapshot `json:"telemetry"`
	DeployMeta   map[string]cockpit.AppMeta  `json:"deploy_meta"`
	DeployEvents []cockpit.DeployEvent       `json:"deploy_events"`
	Registry     Registry                    `json:"registry"`
	Provision    map[string]ProvisionUI      `json:"provision"`
}

func (r *MetricsRing) Board(reg Registry) CockpitBoard {
	meta, events := loadDeployMeta(r.wsRoot)
	enrichDeployMetaFromLaunch(r.wsRoot, r.ws, meta)
	tel := map[string]cockpit.Snapshot{}
	for name, dir := range r.appPaths {
		tel[name] = r.loadCockpit(name, dir)
	}
	history := r.Snapshot()
	for i := range history {
		history[i].Telemetry = nil
	}
	prov := provisionFromState(r.wsRoot)
	return CockpitBoard{
		At:           time.Now().UTC(),
		History:      history,
		Latest:       r.Latest(),
		Telemetry:    tel,
		DeployMeta:   meta,
		DeployEvents: events,
		Registry:     reg,
		Provision:    prov,
	}
}

func provisionFromState(wsRoot string) map[string]ProvisionUI {
	out := map[string]ProvisionUI{}
	st, err := state.Open(wsRoot)
	if err != nil {
		return out
	}
	defer st.Close()
	apps, err := st.ListApps()
	if err != nil {
		return out
	}
	for _, a := range apps {
		if !a.Present {
			continue
		}
		ui := ProvisionUI{Ready: a.Converged, Status: "ready"}
		if !a.Converged {
			ui.Status = "incomplete"
			if a.LastError != "" {
				ui.Reasons = []string{a.LastError}
			} else if a.ReconcileStatus == "running" && a.CurrentStage != "" {
				ui.Reasons = []string{"Reconciling: " + a.CurrentStage}
			} else {
				ui.Reasons = []string{"Reconcile pending"}
			}
		}
		if a.CurrentCommit != "" && a.LastGoodCommit != "" && a.CurrentCommit == a.LastGoodCommit && !a.Converged && a.LastError != "" {
			ui.Reasons = []string{a.LastError}
		}
		out[a.ID] = ui
	}
	return out
}
