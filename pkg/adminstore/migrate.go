package adminstore

import (
	"database/sql"
	"fmt"
)

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE COLLATE NOCASE,
			password_hash TEXT NOT NULL,
			is_root INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS grants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			app_name TEXT NOT NULL,
			permission TEXT NOT NULL,
			UNIQUE(user_id, app_name, permission)
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			action TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("adminstore migrate: %w", err)
		}
	}
	return nil
}
