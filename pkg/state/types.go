package state

import (
	"encoding/json"
	"time"
)

// AppDesired is the intended lifecycle for one app.
type AppDesired struct {
	ID            string
	Present       bool
	Repo          string
	Branch        string
	DeployMode    string
	Zone          string
	PageSubdomain string
	APISubdomain  string
	AppPath       string
	GitRoot       string
}

// AppView is desired + runtime summary for API/UI.
type AppView struct {
	AppDesired
	Converged       bool   `json:"converged"`
	LastError       string `json:"last_error,omitempty"`
	CurrentCommit   string `json:"current_commit,omitempty"`
	LastGoodCommit  string `json:"last_good_commit,omitempty"`
	ReconcileStatus string `json:"reconcile_status,omitempty"`
	CurrentStage    string `json:"current_stage,omitempty"`
	StageIndex      int    `json:"stage_index,omitempty"`
	StageTotal      int    `json:"stage_total,omitempty"`
	RunID           string `json:"run_id,omitempty"`
	LogPath         string `json:"log_path,omitempty"`
	PublicURL       string `json:"public_url,omitempty"`
	APIURL          string `json:"api_url,omitempty"`
}

// JSON field names for embedded AppDesired.
func (a AppView) MarshalJSON() ([]byte, error) {
	type alias struct {
		ID              string `json:"id"`
		Present         bool   `json:"present"`
		Repo            string `json:"repo,omitempty"`
		Branch          string `json:"branch,omitempty"`
		DeployMode      string `json:"deploy_mode,omitempty"`
		Zone            string `json:"zone,omitempty"`
		PageSubdomain   string `json:"page_subdomain,omitempty"`
		APISubdomain    string `json:"api_subdomain,omitempty"`
		AppPath         string `json:"app_path,omitempty"`
		Converged       bool   `json:"converged"`
		LastError       string `json:"last_error,omitempty"`
		CurrentCommit   string `json:"current_commit,omitempty"`
		LastGoodCommit  string `json:"last_good_commit,omitempty"`
		ReconcileStatus string `json:"reconcile_status,omitempty"`
		CurrentStage    string `json:"current_stage,omitempty"`
		StageIndex      int    `json:"stage_index,omitempty"`
		StageTotal      int    `json:"stage_total,omitempty"`
		RunID           string `json:"run_id,omitempty"`
		LogPath         string `json:"log_path,omitempty"`
		PublicURL       string `json:"public_url,omitempty"`
		APIURL          string `json:"api_url,omitempty"`
	}
	return json.Marshal(alias{
		ID: a.ID, Present: a.Present, Repo: a.Repo, Branch: a.Branch, DeployMode: a.DeployMode,
		Zone: a.Zone, PageSubdomain: a.PageSubdomain, APISubdomain: a.APISubdomain, AppPath: a.AppPath,
		Converged: a.Converged, LastError: a.LastError, CurrentCommit: a.CurrentCommit, LastGoodCommit: a.LastGoodCommit,
		ReconcileStatus: a.ReconcileStatus, CurrentStage: a.CurrentStage, StageIndex: a.StageIndex, StageTotal: a.StageTotal,
		RunID: a.RunID, LogPath: a.LogPath, PublicURL: a.PublicURL, APIURL: a.APIURL,
	})
}

// OwnedResource is a Cloudflare or filesystem resource CoGS created.
type OwnedResource struct {
	Kind       string
	ExternalID string
	MetaJSON   string
}

// Release is one immutable build artifact set.
type Release struct {
	ID         int64
	CommitSHA  string
	BuiltAt    time.Time
	BinaryPath string
	HealthOK   bool
}
