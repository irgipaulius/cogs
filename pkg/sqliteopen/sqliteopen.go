// Package sqliteopen opens modernc SQLite with CoGS pragmas and reader/writer pools.
package sqliteopen

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

const defaultDSNSuffix = "_txlock=immediate&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=cache_size(-65536)&_pragma=mmap_size(1073741824)&_pragma=temp_store(MEMORY)"

// Config controls pool sizes for a SQLite file.
type Config struct {
	Path           string
	ReaderConns    int
	PageSize       int // set before first table; 0 = default 4096
}

// Pools holds separate writer and reader *sql.DB handles to the same file.
type Pools struct {
	Writer *sql.DB
	Reader *sql.DB
}

// Open creates writer (MaxOpenConns=1) and reader pools with CoGS pragmas.
func Open(cfg Config) (*Pools, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("sqliteopen: empty path")
	}
	if cfg.ReaderConns <= 0 {
		cfg.ReaderConns = 4
	}
	dsn := "file:" + cfg.Path + "?" + defaultDSNSuffix
	if cfg.PageSize > 0 {
		dsn += fmt.Sprintf("&_pragma=page_size(%d)", cfg.PageSize)
	}

	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)

	reader, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	reader.SetMaxOpenConns(cfg.ReaderConns)
	reader.SetMaxIdleConns(cfg.ReaderConns)

	if err := pingBoth(writer, reader); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, err
	}
	return &Pools{Writer: writer, Reader: reader}, nil
}

func pingBoth(w, r *sql.DB) error {
	if err := w.Ping(); err != nil {
		return err
	}
	return r.Ping()
}

// Close closes both pools.
func (p *Pools) Close() error {
	var errs []string
	if p.Writer != nil {
		if err := p.Writer.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if p.Reader != nil {
		if err := p.Reader.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("sqliteopen close: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Health runs SELECT 1 on the reader pool.
func Health(ctx context.Context, db *sql.DB) error {
	var one int
	return db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

// JournalMode returns the current journal_mode pragma.
func JournalMode(ctx context.Context, db *sql.DB) (string, error) {
	var mode string
	err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode)
	return mode, err
}
