package state

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SetDesired upserts app desired state.
func (s *Store) SetDesired(d AppDesired) error {
	d.ID = strings.TrimSpace(d.ID)
	if d.ID == "" {
		return fmt.Errorf("app id required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	present := 0
	if d.Present {
		present = 1
	}
	return s.tx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
INSERT INTO apps(id, desired_present, repo, branch, deploy_mode, zone, page_subdomain, api_subdomain, app_path, git_root, updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
  desired_present=excluded.desired_present,
  repo=excluded.repo,
  branch=excluded.branch,
  deploy_mode=excluded.deploy_mode,
  zone=excluded.zone,
  page_subdomain=excluded.page_subdomain,
  api_subdomain=excluded.api_subdomain,
  app_path=excluded.app_path,
  git_root=excluded.git_root,
  updated_at=excluded.updated_at`,
			d.ID, present, d.Repo, d.Branch, d.DeployMode, d.Zone, d.PageSubdomain, d.APISubdomain, d.AppPath, d.GitRoot, now)
		return err
	})
}

// GetApp returns one app view.
func (s *Store) GetApp(id string) (AppView, error) {
	apps, err := s.ListApps()
	if err != nil {
		return AppView{}, err
	}
	for _, a := range apps {
		if a.ID == id {
			return a, nil
		}
	}
	return AppView{}, sql.ErrNoRows
}

// ListApps returns all apps with latest reconcile summary.
func (s *Store) ListApps() ([]AppView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`
SELECT a.id, a.desired_present, a.repo, a.branch, a.deploy_mode, a.zone, a.page_subdomain, a.api_subdomain,
       a.app_path, a.git_root, a.converged, a.last_error,
       cr.commit_sha, lg.commit_sha,
       rr.run_id, rr.status, rr.current_stage, rr.log_path
FROM apps a
LEFT JOIN app_release_pointers p ON p.app_id = a.id
LEFT JOIN app_releases cr ON cr.id = p.current_release_id
LEFT JOIN app_releases lg ON lg.id = p.last_good_release_id
LEFT JOIN reconcile_runs rr ON rr.run_id = (
  SELECT run_id FROM reconcile_runs WHERE app_id = a.id ORDER BY started_at DESC LIMIT 1
)
ORDER BY a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppView
	for rows.Next() {
		var v AppView
		var present, converged int
		var cur, lg, runID, status, stage, logPath sql.NullString
		if err := rows.Scan(
			&v.ID, &present, &v.Repo, &v.Branch, &v.DeployMode, &v.Zone, &v.PageSubdomain, &v.APISubdomain,
			&v.AppPath, &v.GitRoot, &converged, &v.LastError,
			&cur, &lg, &runID, &status, &stage, &logPath,
		); err != nil {
			return nil, err
		}
		v.Present = present == 1
		v.Converged = converged == 1
		v.CurrentCommit = cur.String
		v.LastGoodCommit = lg.String
		v.RunID = runID.String
		v.ReconcileStatus = status.String
		v.CurrentStage = stage.String
		v.LogPath = logPath.String
		if v.CurrentStage != "" {
			for i, n := range StageNames {
				if n == v.CurrentStage {
					v.StageIndex = i + 1
					break
				}
			}
		}
		v.StageTotal = len(StageNames)
		if v.Zone != "" {
			if ph := pageHost(v.PageSubdomain, v.Zone); ph != "" {
				v.PublicURL = "https://" + ph
			}
			if ah := apiHost(v.APISubdomain, v.Zone, v.PageSubdomain); ah != "" {
				v.APIURL = "https://" + ah
			}
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func pageHost(sub, zone string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return ""
	}
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return zone
	}
	return sub + "." + zone
}

func apiHost(apiSub, zone, pageSub string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return ""
	}
	apiSub = strings.TrimSpace(apiSub)
	if apiSub == "" {
		return ""
	}
	return apiSub + "." + zone
}

// AppByRepo finds an app id for owner/name repo slug.
func (s *Store) AppByRepo(repoFull string) (string, error) {
	repoFull = strings.ToLower(strings.TrimSpace(repoFull))
	var id string
	err := s.db.QueryRow(`SELECT id FROM apps WHERE lower(repo)=? AND desired_present=1 LIMIT 1`, repoFull).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// SetConverged updates convergence flag and last error.
func (s *Store) SetConverged(appID string, converged bool, lastErr string) error {
	c := 0
	if converged {
		c = 1
	}
	_, err := s.db.Exec(`UPDATE apps SET converged=?, last_error=?, updated_at=? WHERE id=?`,
		c, lastErr, time.Now().UTC().Format(time.RFC3339), appID)
	if err == nil {
		s.notify()
	}
	return err
}

// ClearHostnames removes hostname claims for an app.
func (s *Store) ClearHostnames(appID string) error {
	_, err := s.db.Exec(`DELETE FROM hostnames WHERE app_id=?`, appID)
	if err == nil {
		s.notify()
	}
	return err
}

// DeleteApp removes an app row (after GC).
func (s *Store) DeleteApp(appID string) error {
	return s.tx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM apps WHERE id=?`, appID)
		return err
	})
}

// ClaimHostname registers a hostname for an app (page or api).
func (s *Store) ClaimHostname(appID, hostname, kind string) error {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	return s.tx(func(tx *sql.Tx) error {
		var other string
		err := tx.QueryRow(`SELECT app_id FROM hostnames WHERE hostname=?`, hostname).Scan(&other)
		if err == nil && other != appID {
			return fmt.Errorf("hostname %q already used by app %q", hostname, other)
		}
		if err == sql.ErrNoRows {
			_, err = tx.Exec(`INSERT INTO hostnames(hostname, app_id, kind) VALUES(?,?,?)`, hostname, appID, kind)
		}
		return err
	})
}

// CheckHostnames returns an error if page or api host conflict with another app.
func (s *Store) CheckHostnames(appID, pageSub, apiSub, zone string) error {
	appID = strings.TrimSpace(appID)
	if ph := pageHost(pageSub, zone); ph != "" {
		var other string
		err := s.db.QueryRow(`SELECT app_id FROM hostnames WHERE hostname=?`, strings.ToLower(ph)).Scan(&other)
		if err == nil && other != appID {
			return fmt.Errorf("UI hostname %s used by %q", ph, other)
		}
	}
	if ah := apiHost(apiSub, zone, pageSub); ah != "" {
		var other string
		err := s.db.QueryRow(`SELECT app_id FROM hostnames WHERE hostname=?`, strings.ToLower(ah)).Scan(&other)
		if err == nil && other != appID {
			return fmt.Errorf("API hostname %s used by %q", ah, other)
		}
	}
	return nil
}
