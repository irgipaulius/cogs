package supervisor

import (
	"time"

	"github.com/irgipaulius/cogs/pkg/cockpit"
)

// CockpitBoard is the full admin dashboard payload.
type CockpitBoard struct {
	At           time.Time                      `json:"at"`
	History      []MetricsSample                `json:"history"`
	Latest       MetricsSample                  `json:"latest"`
	Telemetry    map[string]cockpit.Snapshot    `json:"telemetry"`
	DeployMeta   map[string]cockpit.AppMeta     `json:"deploy_meta"`
	DeployEvents []cockpit.DeployEvent          `json:"deploy_events"`
	Registry     Registry                       `json:"registry"`
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
	return CockpitBoard{
		At:           time.Now().UTC(),
		History:      history,
		Latest:       r.Latest(),
		Telemetry:    tel,
		DeployMeta:   meta,
		DeployEvents: events,
		Registry:     reg,
	}
}
