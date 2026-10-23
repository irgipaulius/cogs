package state

import (
	"database/sql"
	"time"
)

// UpsertRelease records a built release.
func (s *Store) UpsertRelease(appID, commit, binaryPath string, healthOK bool) (int64, error) {
	var id int64
	h := 0
	if healthOK {
		h = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.tx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
INSERT INTO app_releases(app_id, commit_sha, built_at, binary_path, health_ok)
VALUES(?,?,?,?,?)
ON CONFLICT(app_id, commit_sha) DO UPDATE SET binary_path=excluded.binary_path, health_ok=excluded.health_ok`,
			appID, commit, now, binaryPath, h)
		if err != nil {
			return err
		}
		return tx.QueryRow(`SELECT id FROM app_releases WHERE app_id=? AND commit_sha=?`, appID, commit).Scan(&id)
	})
	return id, err
}

// ActivateRelease sets current and last_good to release id (after health pass).
func (s *Store) ActivateRelease(appID string, releaseID int64) error {
	return s.tx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
INSERT INTO app_release_pointers(app_id, current_release_id, last_good_release_id)
VALUES(?,?,?)
ON CONFLICT(app_id) DO UPDATE SET current_release_id=excluded.current_release_id, last_good_release_id=excluded.last_good_release_id`,
			appID, releaseID, releaseID)
		return err
	})
}

// CurrentRelease returns current commit sha if set.
func (s *Store) CurrentRelease(appID string) (commit string, binaryPath string, err error) {
	err = s.db.QueryRow(`
SELECT r.commit_sha, r.binary_path FROM app_release_pointers p
JOIN app_releases r ON r.id = p.current_release_id
WHERE p.app_id=?`, appID).Scan(&commit, &binaryPath)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return commit, binaryPath, err
}

// ReleaseRow is one release DB row.
type ReleaseRow struct {
	ID        int64
	CommitSHA string
}

// ListReleaseRows returns releases newest first.
func (s *Store) ListReleaseRows(appID string) ([]ReleaseRow, error) {
	rows, err := s.db.Query(`SELECT id, commit_sha FROM app_releases WHERE app_id=? ORDER BY built_at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReleaseRow
	for rows.Next() {
		var r ReleaseRow
		if err := rows.Scan(&r.ID, &r.CommitSHA); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// KeepReleaseSHAs returns commit SHAs to retain (current, last_good, up to limit newest).
func (s *Store) KeepReleaseSHAs(appID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 3
	}
	cur, _, _ := s.CurrentRelease(appID)
	lg, _ := s.LastGoodRelease(appID)
	rels, err := s.ListReleaseRows(appID)
	if err != nil {
		return nil, err
	}
	keep := map[string]bool{}
	if cur != "" {
		keep[cur] = true
	}
	if lg != "" {
		keep[lg] = true
	}
	for _, r := range rels {
		if len(keep) >= limit {
			break
		}
		keep[r.CommitSHA] = true
	}
	out := make([]string, 0, len(keep))
	for sha := range keep {
		out = append(out, sha)
	}
	return out, nil
}

// LastGoodRelease returns last good commit sha.
func (s *Store) LastGoodRelease(appID string) (string, error) {
	var commit string
	err := s.db.QueryRow(`
SELECT r.commit_sha FROM app_release_pointers p
JOIN app_releases r ON r.id = p.last_good_release_id
WHERE p.app_id=?`, appID).Scan(&commit)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return commit, err
}

// DeleteReleaseRow removes one release row.
func (s *Store) DeleteReleaseRow(appID string, id int64) error {
	_, err := s.db.Exec(`DELETE FROM app_releases WHERE app_id=? AND id=?`, appID, id)
	return err
}

// ListReleaseIDs returns release row ids newest first.
func (s *Store) ListReleaseIDs(appID string, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`SELECT id FROM app_releases WHERE app_id=? ORDER BY built_at DESC LIMIT ?`, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
