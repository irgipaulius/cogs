package installjob

import "time"

// Status of an install job or stage.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// StageID is a fixed pipeline step.
type StageID string

const (
	StageClone    StageID = "clone"
	StageDiscover StageID = "discover"
	StagePrepare  StageID = "prepare"
	StageRegister StageID = "register"
	StageBuild    StageID = "build"
	StageLaunch   StageID = "launch"
	StageHealth   StageID = "health"
	StageDone     StageID = "done"
)

// StageOrder is the install pipeline (excluding terminal "done").
var StageOrder = []StageID{
	StageClone,
	StageDiscover,
	StagePrepare,
	StageRegister,
	StageBuild,
	StageLaunch,
	StageHealth,
	StageDone,
}

func (s StageID) Label() string {
	switch s {
	case StageClone:
		return "Cloning repository"
	case StageDiscover:
		return "Reading configuration"
	case StagePrepare:
		return "Preparing workspace"
	case StageRegister:
		return "Registering app"
	case StageBuild:
		return "Building backend"
	case StageLaunch:
		return "Provisioning & deploy"
	case StageHealth:
		return "Health check"
	case StageDone:
		return "Complete"
	default:
		return string(s)
	}
}

// Stage is one pipeline step with persisted progress.
type Stage struct {
	ID        StageID   `json:"id"`
	Status    Status    `json:"status"`
	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	OutputTail string   `json:"output_tail,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Request is the user input for an install.
type Request struct {
	RepoFull       string `json:"repo"`
	AppName        string `json:"name"`
	DeployMode     string `json:"deploy_mode"`
	PageSubdomain  string `json:"page_subdomain"`
	PageApex       bool   `json:"page_apex"`
	Zone           string `json:"zone"`
	ScaffoldOnly   bool   `json:"scaffold_only"`
	FinishApp      string `json:"finish_app,omitempty"` // provision an already-registered app
}

// Job is a persisted install job.
type Job struct {
	ID        string    `json:"id"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Request   Request   `json:"request"`

	AppName   string `json:"app_name,omitempty"`
	AppDir    string `json:"app_dir,omitempty"`
	GitRoot   string `json:"git_root,omitempty"`
	LogPath   string `json:"log_path,omitempty"`

	CurrentStage StageID `json:"current_stage,omitempty"`
	Stages       []Stage `json:"stages"`

	Error string `json:"error,omitempty"`

	// SideEffects tracks what this job alone changed (for rollback).
	SideEffects JobSideEffects `json:"side_effects,omitempty"`
}

// JobSideEffects records mutations to undo on failure.
type JobSideEffects struct {
	Registered bool   `json:"registered,omitempty"`
	AppPath    string `json:"app_path,omitempty"`
}

func (j *Job) stageIndex(id StageID) int {
	for i, s := range j.Stages {
		if s.ID == id {
			return i
		}
	}
	return -1
}

func (j *Job) progress() (current int, total int) {
	total = len(StageOrder)
	if j.CurrentStage == "" {
		return 0, total
	}
	for i, s := range StageOrder {
		if s == j.CurrentStage {
			return i + 1, total
		}
	}
	return 0, total
}
