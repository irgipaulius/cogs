// Package jobs provides SQLite-backed job locking and a simple registry.
package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Registry maps job names to handlers.
type Registry map[string]func(context.Context) error

// EnsureSchema creates the jobs metadata table.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _cogs_jobs (
			name TEXT PRIMARY KEY,
			last_run_at TEXT,
			last_error TEXT,
			lock_until TEXT
		)`)
	return err
}

// TryLock acquires a job lock until expiry.
func TryLock(ctx context.Context, db *sql.DB, name string, ttl time.Duration) (bool, error) {
	until := time.Now().Add(ttl).UTC().Format(time.RFC3339Nano)
	res, err := db.ExecContext(ctx, `
		INSERT INTO _cogs_jobs (name, lock_until) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET lock_until=excluded.lock_until
		WHERE _cogs_jobs.lock_until IS NULL OR _cogs_jobs.lock_until < datetime('now')`,
		name, until)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Unlock clears the lock and records run outcome.
func Unlock(ctx context.Context, db *sql.DB, name string, runErr error) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	errMsg := ""
	if runErr != nil {
		errMsg = runErr.Error()
	}
	_, err := db.ExecContext(ctx, `
		UPDATE _cogs_jobs SET lock_until=NULL, last_run_at=?, last_error=? WHERE name=?`,
		now, errMsg, name)
	return err
}

// Run executes a named job with overlap protection.
func Run(ctx context.Context, db *sql.DB, reg Registry, name string, lockTTL time.Duration) error {
	h, ok := reg[name]
	if !ok {
		return fmt.Errorf("jobs: unknown job %q", name)
	}
	okLock, err := TryLock(ctx, db, name, lockTTL)
	if err != nil {
		return err
	}
	if !okLock {
		return fmt.Errorf("jobs: %q already running", name)
	}
	runErr := h(ctx)
	_ = Unlock(ctx, db, name, runErr)
	return runErr
}
