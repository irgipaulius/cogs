package supervisor

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const sqlMaxRows = 500
const sqlTimeout = 10 * time.Second

// SQLResult is a query outcome.
type SQLResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	RowsAffected int64      `json:"rows_affected,omitempty"`
}

// RunSQL executes one statement against a registered database id.
func RunSQL(ctx context.Context, reg Registry, dbID, statement string, write bool) (SQLResult, error) {
	var target *AppDB
	for i := range reg.Apps {
		if reg.Apps[i].ID == dbID {
			target = &reg.Apps[i]
			break
		}
	}
	if target == nil {
		return SQLResult{}, fmt.Errorf("unknown database id %q", dbID)
	}
	stmt := strings.TrimSpace(statement)
	if stmt == "" {
		return SQLResult{}, fmt.Errorf("empty statement")
	}
	upper := strings.ToUpper(stmt)
	if !write {
		if !isReadOnlySQL(upper) {
			return SQLResult{}, fmt.Errorf("read-only: use SELECT, WITH, PRAGMA, or EXPLAIN")
		}
	} else {
		if strings.HasPrefix(upper, "SELECT") {
			return SQLResult{}, fmt.Errorf("write mode: SELECT not allowed")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()
	db, err := sql.Open("sqlite", target.Path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return SQLResult{}, err
	}
	defer db.Close()

	if write || !strings.HasPrefix(upper, "SELECT") {
		res, err := db.ExecContext(ctx, stmt)
		if err != nil {
			return SQLResult{}, err
		}
		n, _ := res.RowsAffected()
		return SQLResult{RowsAffected: n}, nil
	}

	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return SQLResult{}, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return SQLResult{}, err
	}
	var out [][]interface{}
	for rows.Next() {
		if len(out) >= sqlMaxRows {
			break
		}
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return SQLResult{}, err
		}
		row := make([]interface{}, len(cols))
		for i, v := range vals {
			switch x := v.(type) {
			case []byte:
				row[i] = string(x)
			default:
				row[i] = x
			}
		}
		out = append(out, row)
	}
	return SQLResult{Columns: cols, Rows: out}, rows.Err()
}

// ListTables returns user tables for a db id.
func ListTables(ctx context.Context, reg Registry, dbID string) ([]string, error) {
	res, err := RunSQL(ctx, reg, dbID, `
		SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name
	`, false)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, row := range res.Rows {
		if len(row) > 0 {
			names = append(names, fmt.Sprint(row[0]))
		}
	}
	return names, nil
}

func isReadOnlySQL(upper string) bool {
	switch {
	case strings.HasPrefix(upper, "SELECT"),
		strings.HasPrefix(upper, "WITH"),
		strings.HasPrefix(upper, "PRAGMA"),
		strings.HasPrefix(upper, "EXPLAIN"):
		return true
	default:
		return false
	}
}
