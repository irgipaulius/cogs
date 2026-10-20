//go:build ignore

package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// MigrateUp applies pending migrations and returns the current version.
func MigrateUp(ctx context.Context, db *sql.DB) (int64, error) {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		return 0, err
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return 0, err
	}
	v, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// MigrateDown rolls back one version (dev helper).
func MigrateDown(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		return err
	}
	return goose.DownContext(ctx, db, "migrations")
}

// CurrentVersion returns goose version without migrating.
func CurrentVersion(ctx context.Context, db *sql.DB) (int64, error) {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		return 0, err
	}
	return goose.GetDBVersionContext(ctx, db)
}

// SQLCMarker documents sqlc integration point.
func SQLCMarker() string {
	return fmt.Sprintf("sqlc queries live in query/")
}
