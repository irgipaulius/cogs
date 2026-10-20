package observe

import (
	"context"
	"database/sql"
	"os"
	"time"
)

// DBSnapshot describes SQLite file + optional counters.
type DBSnapshot struct {
	Path       string  `json:"path"`
	SizeBytes  int64   `json:"size_bytes"`
	Modified   string  `json:"modified_utc"`
	PageCount  int64   `json:"page_count"`
	PageSize   int64   `json:"page_size"`
	SQLQueries uint64  `json:"sql_queries"`
	SQLExecs   uint64  `json:"sql_execs"`
}

func dbSnapshot(path string, db *sql.DB, sql *SQLStats) DBSnapshot {
	out := DBSnapshot{Path: path}
	if fi, err := os.Stat(path); err == nil {
		out.SizeBytes = fi.Size()
		out.Modified = fi.ModTime().UTC().Format(time.RFC3339)
	}
	if db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&out.PageCount)
		_ = db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&out.PageSize)
	}
	if sql != nil {
		out.SQLQueries = sql.Queries.Load()
		out.SQLExecs = sql.Execs.Load()
	}
	return out
}
