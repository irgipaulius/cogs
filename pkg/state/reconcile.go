package state

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

// StageNames align with pkg/reconcile.
var StageNames = []string{
	"source", "validate", "build_workspace", "secrets",
	"cf_resources", "tunnel", "worker", "dns",
	"build_release", "migrate", "activate", "gc",
}

// BeginRun creates a reconcile run row.
func (s *Store) BeginRun(appID, logPath string) (runID string, err error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	runID = hex.EncodeToString(b[:])
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.Exec(`
INSERT INTO reconcile_runs(run_id, app_id, status, current_stage, started_at, log_path)
VALUES(?,?,'running','source',?,?)`, runID, appID, now, logPath)
	if err == nil {
		s.notify()
	}
	return runID, err
}

// UpdateRunStage updates current stage on the run.
func (s *Store) UpdateRunStage(runID, stage, status string) error {
	_, err := s.db.Exec(`UPDATE reconcile_runs SET current_stage=?, status=? WHERE run_id=?`, stage, status, runID)
	if err == nil {
		s.notify()
	}
	return err
}

// FinishRun marks run terminal state.
func (s *Store) FinishRun(runID, status, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE reconcile_runs SET status=?, ended_at=?, error=? WHERE run_id=?`, status, now, errMsg, runID)
	if err == nil {
		s.notify()
	}
	return err
}

// RecordStage appends stage progress.
func (s *Store) RecordStage(runID, name, status, outputTail, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
INSERT INTO reconcile_stages(run_id, stage_name, status, started_at, ended_at, output_tail, error)
VALUES(?,?,?,?,?,?,?)`, runID, name, status, now, now, outputTail, errMsg)
	if err == nil {
		s.notify()
	}
	return err
}

// RecordOwned saves an owned resource.
func (s *Store) RecordOwned(appID, kind, externalID, metaJSON string) error {
	return s.tx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
INSERT INTO app_owned(app_id, kind, external_id, meta_json) VALUES(?,?,?,?)
ON CONFLICT(app_id, kind, external_id) DO UPDATE SET meta_json=excluded.meta_json`,
			appID, kind, externalID, metaJSON)
		return err
	})
}

// ListOwned returns owned resources for GC.
func (s *Store) ListOwned(appID string) ([]OwnedResource, error) {
	rows, err := s.db.Query(`SELECT kind, external_id, meta_json FROM app_owned WHERE app_id=?`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OwnedResource
	for rows.Next() {
		var r OwnedResource
		if err := rows.Scan(&r.Kind, &r.ExternalID, &r.MetaJSON); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClearOwned removes all owned rows for an app after GC.
func (s *Store) ClearOwned(appID string) error {
	_, err := s.db.Exec(`DELETE FROM app_owned WHERE app_id=?`, appID)
	return err
}
