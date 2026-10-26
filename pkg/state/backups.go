package state

import (
	"database/sql"
	"fmt"
	"time"
)

// BackupRow is one catalog entry.
type BackupRow struct {
	ID        int64
	AppID     string
	RelPath   string
	CreatedAt time.Time
	Bytes     int64
	SHA256    string
	Driver    string
	Error     string
}

// InsertBackup records a successful snapshot.
func (s *Store) InsertBackup(appID, relPath, driver string, bytes int64, sha256 string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var id int64
	err := s.db.QueryRow(`
INSERT INTO app_backups(app_id, rel_path, created_at, bytes, sha256, driver, error)
VALUES(?,?,?,?,?,?,?)
RETURNING id`, appID, relPath, now, bytes, sha256, driver, "").Scan(&id)
	if err == nil {
		s.notify()
	}
	return id, err
}

// ListBackups returns newest first.
func (s *Store) ListBackups(appID string, limit int) ([]BackupRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
SELECT id, app_id, rel_path, created_at, bytes, sha256, driver, error
FROM app_backups WHERE app_id=? AND error='' ORDER BY created_at DESC LIMIT ?`, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupRow
	for rows.Next() {
		var r BackupRow
		var created string
		if err := rows.Scan(&r.ID, &r.AppID, &r.RelPath, &created, &r.Bytes, &r.SHA256, &r.Driver, &r.Error); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		if r.CreatedAt.IsZero() {
			r.CreatedAt, _ = time.Parse(time.RFC3339, created)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetBackup returns one row by id for app.
func (s *Store) GetBackup(appID string, id int64) (BackupRow, error) {
	var r BackupRow
	var created string
	err := s.db.QueryRow(`
SELECT id, app_id, rel_path, created_at, bytes, sha256, driver, error
FROM app_backups WHERE app_id=? AND id=?`, appID, id).Scan(
		&r.ID, &r.AppID, &r.RelPath, &created, &r.Bytes, &r.SHA256, &r.Driver, &r.Error)
	if err == sql.ErrNoRows {
		return BackupRow{}, fmt.Errorf("backup not found")
	}
	if err != nil {
		return BackupRow{}, err
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return r, nil
}

// LatestBackupTime returns the newest successful backup time.
func (s *Store) LatestBackupTime(appID string) (time.Time, error) {
	var created string
	err := s.db.QueryRow(`
SELECT created_at FROM app_backups WHERE app_id=? AND error='' ORDER BY created_at DESC LIMIT 1`, appID).Scan(&created)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, created)
	}
	return t, nil
}

// DeleteBackupRow removes catalog row (file cleanup is separate).
func (s *Store) DeleteBackupRow(id int64) error {
	_, err := s.db.Exec(`DELETE FROM app_backups WHERE id=?`, id)
	if err == nil {
		s.notify()
	}
	return err
}

// ListBackupIDsOlderThan returns ids beyond keep limit (newest first ordering).
func (s *Store) ListBackupIDsOlderThan(appID string, keep int) ([]int64, error) {
	rows, err := s.db.Query(`
SELECT id FROM app_backups WHERE app_id=? AND error='' ORDER BY created_at DESC`, appID)
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
	if len(ids) <= keep {
		return nil, rows.Err()
	}
	return ids[keep:], rows.Err()
}
