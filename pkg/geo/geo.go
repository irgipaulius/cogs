// Package geo provides bbox queries with R-tree or lat/lon fallback.
package geo

import (
	"context"
	"database/sql"
	"fmt"
)

// Mode reports which index strategy is active.
type Mode string

const (
	ModeRTree Mode = "rtree"
	ModeIndex Mode = "latlon_index"
)

// DetectMode checks whether rtree virtual table works on this SQLite build.
func DetectMode(ctx context.Context, db *sql.DB) Mode {
	_, err := db.ExecContext(ctx, `CREATE VIRTUAL TABLE IF NOT EXISTS _cogs_rtree_probe USING rtree(id, min_x, max_x, min_y, max_y)`)
	if err != nil {
		return ModeIndex
	}
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS _cogs_rtree_probe`)
	return ModeRTree
}

// EnsureSchema creates geo tables for the given mode.
func EnsureSchema(ctx context.Context, db *sql.DB, mode Mode) error {
	if mode == ModeRTree {
		_, err := db.ExecContext(ctx, `
			CREATE VIRTUAL TABLE IF NOT EXISTS geo_rtree USING rtree(
				id, min_x, max_x, min_y, max_y
			)`)
		return err
	}
	_, err := db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS geo_latlon ON geo_points (lat, lon)`)
	return err
}

// QueryBBox returns point ids inside the bounding box.
func QueryBBox(ctx context.Context, db *sql.DB, mode Mode, minLon, minLat, maxLon, maxLat float64) ([]int64, error) {
	if mode == ModeRTree {
		rows, err := db.QueryContext(ctx, `
			SELECT id FROM geo_rtree
			WHERE min_x <= ? AND max_x >= ? AND min_y <= ? AND max_y >= ?`,
			maxLon, minLon, maxLat, minLat)
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
	rows, err := db.QueryContext(ctx, `
		SELECT id FROM geo_points
		WHERE lon BETWEEN ? AND ? AND lat BETWEEN ? AND ?`,
		minLon, maxLon, minLat, maxLat)
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

// UpsertRTree syncs one point into the rtree table (same transaction as caller).
func UpsertRTree(ctx context.Context, tx *sql.Tx, id int64, lon, lat float64) error {
	_, _ = tx.ExecContext(ctx, `DELETE FROM geo_rtree WHERE id = ?`, id)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO geo_rtree (id, min_x, max_x, min_y, max_y) VALUES (?, ?, ?, ?, ?)`,
		id, lon, lon, lat, lat)
	if err != nil {
		return fmt.Errorf("geo rtree upsert: %w", err)
	}
	return nil
}
