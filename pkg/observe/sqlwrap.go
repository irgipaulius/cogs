package observe

import (
	"context"
	"database/sql"
)

// InstrumentDB wraps a *sql.DB to count queries and execs (use for writer pool).
func InstrumentDB(db *sql.DB, stats *SQLStats) *InstrumentedDB {
	if db == nil || stats == nil {
		return &InstrumentedDB{DB: db, Stats: stats}
	}
	return &InstrumentedDB{DB: db, Stats: stats}
}

// InstrumentedDB forwards to sql.DB and updates SQLStats.
type InstrumentedDB struct {
	DB    *sql.DB
	Stats *SQLStats
}

func (d *InstrumentedDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if d.Stats != nil {
		d.Stats.Queries.Add(1)
	}
	return d.DB.QueryContext(ctx, query, args...)
}

func (d *InstrumentedDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if d.Stats != nil {
		d.Stats.Queries.Add(1)
	}
	return d.DB.QueryRowContext(ctx, query, args...)
}

func (d *InstrumentedDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if d.Stats != nil {
		d.Stats.Execs.Add(1)
	}
	return d.DB.ExecContext(ctx, query, args...)
}
