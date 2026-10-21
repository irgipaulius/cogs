package db

import (
	"context"
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func MigrateUp(ctx context.Context, db *sql.DB) (int64, error) {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		return 0, err
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return 0, err
	}
	return goose.GetDBVersionContext(ctx, db)
}
