// Package sync pushes SQLite projections to D1 and R2 using an outbox fast path.
package sync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
)

const defaultBlocks = 512

// Projection describes one sync set from sync.json.
type Projection struct {
	Name         string   `json:"name"`
	Table        string   `json:"table"`
	PrimaryKey   string   `json:"primary_key"`
	Columns      []string `json:"columns"`
	BlobColumn   string   `json:"blob_column,omitempty"`
	D1Table      string   `json:"d1_table"`
	BlockBuckets int      `json:"block_buckets,omitempty"`
}

// Config is the sync.json root.
type Config struct {
	Sets []Projection `json:"sets"`
}

// LoadConfig reads sync.json from path.
func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// D1Client executes SQL against Cloudflare D1 (HTTP API or fake in tests).
type D1Client interface {
	Query(ctx context.Context, sql string, params ...any) ([][]string, error)
	ExecBatch(ctx context.Context, statements []Statement) error
}

// Statement is one D1 SQL statement with bound params (strings only for API).
type Statement struct {
	SQL    string
	Params []string
}

// BlobStore uploads and heads blob objects (R2).
type BlobStore interface {
	HeadHash(ctx context.Context, key string) (hash string, exists bool, err error)
	PutBlob(ctx context.Context, key string, data []byte) (hash string, err error)
}

// Stats from a sync run.
type Stats struct {
	Dirty    int
	Uploaded int
	Skipped  int
	Deleted  int
}

// Engine runs outbox, checksum, or rebuild sync modes.
type Engine struct {
	DB       *sql.DB
	D1       D1Client
	Blobs    BlobStore
	Config   Config
	SetName  string
	AllowDel bool
}

// EnsureSchema creates local outbox/block tables.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS _cogs_outbox (
			set_name TEXT NOT NULL,
			pk TEXT NOT NULL,
			op TEXT NOT NULL,
			row_hash TEXT NOT NULL,
			blob_hash TEXT,
			PRIMARY KEY (set_name, pk)
		)`,
		`CREATE TABLE IF NOT EXISTS _cogs_blocks (
			set_name TEXT NOT NULL,
			block_id INTEGER NOT NULL,
			block_hash TEXT NOT NULL,
			PRIMARY KEY (set_name, block_id)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// PushOutbox uploads pending outbox rows (fast path).
func (e *Engine) PushOutbox(ctx context.Context) (Stats, error) {
	var st Stats
	proj, err := e.projection()
	if err != nil {
		return st, err
	}
	rows, err := e.DB.QueryContext(ctx,
		`SELECT pk, op, row_hash, COALESCE(blob_hash,'') FROM _cogs_outbox WHERE set_name = ?`, e.SetName)
	if err != nil {
		return st, err
	}
	defer rows.Close()

	type item struct {
		pk, op, rowHash, blobHash string
	}
	var pending []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.pk, &it.op, &it.rowHash, &it.blobHash); err != nil {
			return st, err
		}
		pending = append(pending, it)
	}
	st.Dirty = len(pending)

	var batch []Statement
	for _, it := range pending {
		if it.op == "delete" {
			if !e.AllowDel {
				continue
			}
			batch = append(batch, Statement{
				SQL:    fmt.Sprintf(`DELETE FROM %s WHERE %s = ?`, proj.D1Table, proj.PrimaryKey),
				Params: []string{it.pk},
			})
			st.Deleted++
		} else {
			if err := e.uploadBlobIfNeeded(ctx, proj, it.pk, it.blobHash); err != nil {
				return st, err
			}
			if it.blobHash != "" {
				if skipped, err := e.blobSkipped(ctx, proj, it.blobHash); err != nil {
					return st, err
				} else if skipped {
					st.Skipped++
				}
			}
			stmt, err := e.upsertStatement(ctx, proj, it.pk, it.rowHash)
			if err != nil {
				return st, err
			}
			batch = append(batch, stmt)
			st.Uploaded++
		}
	}
	if len(batch) > 0 && e.D1 != nil {
		if err := e.D1.ExecBatch(ctx, batch); err != nil {
			return st, err
		}
	}
	for _, it := range pending {
		if it.op == "delete" && !e.AllowDel {
			continue
		}
		_, _ = e.DB.ExecContext(ctx, `DELETE FROM _cogs_outbox WHERE set_name = ? AND pk = ?`, e.SetName, it.pk)
	}
	return st, nil
}

func (e *Engine) projection() (Projection, error) {
	for _, p := range e.Config.Sets {
		if p.Name == e.SetName {
			if p.BlockBuckets <= 0 {
				p.BlockBuckets = defaultBlocks
			}
			if p.D1Table == "" {
				p.D1Table = p.Table + "_edge"
			}
			return p, nil
		}
	}
	return Projection{}, fmt.Errorf("sync: unknown set %q", e.SetName)
}

func (e *Engine) uploadBlobIfNeeded(ctx context.Context, p Projection, pk, blobHash string) error {
	if p.BlobColumn == "" || blobHash == "" || e.Blobs == nil {
		return nil
	}
	key := blobKey(e.SetName, blobHash)
	h, ok, err := e.Blobs.HeadHash(ctx, key)
	if err != nil {
		return err
	}
	if ok && h == blobHash {
		return nil
	}
	var blob []byte
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE %s = ?`, p.BlobColumn, p.Table, p.PrimaryKey)
	if err := e.DB.QueryRowContext(ctx, q, pk).Scan(&blob); err != nil {
		return err
	}
	_, err = e.Blobs.PutBlob(ctx, key, blob)
	return err
}

func (e *Engine) blobSkipped(ctx context.Context, p Projection, blobHash string) (bool, error) {
	if blobHash == "" || e.Blobs == nil {
		return false, nil
	}
	key := blobKey(e.SetName, blobHash)
	h, ok, err := e.Blobs.HeadHash(ctx, key)
	if err != nil {
		return false, err
	}
	return ok && h == blobHash, nil
}

func blobKey(set, hash string) string {
	return fmt.Sprintf("cogs/%s/%s.zst", set, hash)
}

func (e *Engine) upsertStatement(ctx context.Context, p Projection, pk, rowHash string) (Statement, error) {
	cols := append([]string{p.PrimaryKey}, p.Columns...)
	cols = append(cols, "row_hash")
	placeholders := strings.Repeat("?,", len(cols))
	placeholders = placeholders[:len(placeholders)-1]
	updates := make([]string, 0, len(cols))
	for _, c := range cols {
		if c == p.PrimaryKey {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s=excluded.%s", c, c))
	}
	sqlStmt := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(%s) DO UPDATE SET %s`,
		p.D1Table, strings.Join(cols, ","), placeholders, p.PrimaryKey, strings.Join(updates, ","),
	)
	args := make([]string, 0, len(cols))
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE %s = ?`, strings.Join(append(p.Columns, p.PrimaryKey), ","), p.Table, p.PrimaryKey)
	row := e.DB.QueryRowContext(ctx, q, pk)
	scanDest := make([]any, len(p.Columns)+1)
	var pkVal string
	scanDest[len(p.Columns)] = &pkVal
	for i := range p.Columns {
		var s sql.NullString
		scanDest[i] = &s
	}
	if err := row.Scan(scanDest...); err != nil {
		return Statement{}, err
	}
	args = append(args, pkVal)
	for i := range p.Columns {
		scanDest[i].(*sql.NullString).Valid = true
		args = append(args, scanDest[i].(*sql.NullString).String)
	}
	args = append(args, rowHash)
	return Statement{SQL: sqlStmt, Params: args}, nil
}

// BlockID maps primary key to a bucket 0..blocks-1.
func BlockID(pk string, blocks int) int {
	if blocks <= 0 {
		blocks = defaultBlocks
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(pk))
	return int(h.Sum32() % uint32(blocks))
}

// RowHash computes a stable hash over column values.
func RowHash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// EnqueueOutbox marks a row dirty (app or trigger companion).
func EnqueueOutbox(ctx context.Context, db *sql.DB, set, pk, op, rowHash, blobHash string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO _cogs_outbox (set_name, pk, op, row_hash, blob_hash)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(set_name, pk) DO UPDATE SET
			op=excluded.op, row_hash=excluded.row_hash, blob_hash=excluded.blob_hash`,
		set, pk, op, rowHash, nullIfEmpty(blobHash))
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// RebuildOutbox scans the whole table and fills the outbox (slow).
func (e *Engine) RebuildOutbox(ctx context.Context) (int, error) {
	p, err := e.projection()
	if err != nil {
		return 0, err
	}
	q := fmt.Sprintf(`SELECT %s, %s FROM %s`, p.PrimaryKey, strings.Join(p.Columns, ","), p.Table)
	rws, err := e.DB.QueryContext(ctx, q)
	if err != nil {
		return 0, err
	}
	defer rws.Close()
	n := 0
	for rws.Next() {
		vals := make([]sql.NullString, 1+len(p.Columns))
		dest := make([]any, len(vals))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rws.Scan(dest...); err != nil {
			return n, err
		}
		pk := vals[0].String
		parts := []string{pk}
		for i := 1; i < len(vals); i++ {
			parts = append(parts, vals[i].String)
		}
		rh := RowHash(parts...)
		var bh string
		if p.BlobColumn != "" {
			for i, col := range p.Columns {
				if col == p.BlobColumn {
					bh = RowHash(vals[i+1].String)
					break
				}
			}
		}
		if err := EnqueueOutbox(ctx, e.DB, e.SetName, pk, "upsert", rh, bh); err != nil {
			return n, err
		}
		n++
	}
	return n, rws.Err()
}

// ChecksumSync compares block hashes with D1 and enqueues diffs for dirty buckets only.
func (e *Engine) ChecksumSync(ctx context.Context) (Stats, error) {
	p, err := e.projection()
	if err != nil {
		return Stats{}, err
	}
	blocks := p.BlockBuckets
	local := make(map[int]string, blocks)
	for i := 0; i < blocks; i++ {
		local[i] = ""
	}
	q := fmt.Sprintf(`SELECT %s FROM %s`, p.PrimaryKey, p.Table)
	rws, err := e.DB.QueryContext(ctx, q)
	if err != nil {
		return Stats{}, err
	}
	for rws.Next() {
		var pk string
		if err := rws.Scan(&pk); err != nil {
			rws.Close()
			return Stats{}, err
		}
		bid := BlockID(pk, blocks)
		local[bid] = RowHash(local[bid], pk)
	}
	rws.Close()

	remote := make(map[int]string, blocks)
	if e.D1 != nil {
		rows, err := e.D1.Query(ctx, `SELECT block_id, block_hash FROM _cogs_block_hashes WHERE set_name = ?`, e.SetName)
		if err == nil {
			for _, r := range rows {
				if len(r) >= 2 {
					var id int
					fmt.Sscanf(r[0], "%d", &id)
					remote[id] = r[1]
				}
			}
		}
	}
	allPKs, err := e.collectPrimaryKeys(ctx, q)
	if err != nil {
		return Stats{}, err
	}
	for id, lh := range local {
		if remote[id] == lh {
			continue
		}
		for _, pk := range allPKs {
			if BlockID(pk, blocks) != id {
				continue
			}
			rh := RowHash(pk)
			if err := EnqueueOutbox(ctx, e.DB, e.SetName, pk, "upsert", rh, ""); err != nil {
				return Stats{}, err
			}
		}
		_, _ = e.DB.ExecContext(ctx, `
			INSERT INTO _cogs_blocks (set_name, block_id, block_hash) VALUES (?, ?, ?)
			ON CONFLICT(set_name, block_id) DO UPDATE SET block_hash=excluded.block_hash`,
			e.SetName, id, lh)
	}
	return e.PushOutbox(ctx)
}

func (e *Engine) collectPrimaryKeys(ctx context.Context, q string) ([]string, error) {
	rws, err := e.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rws.Close()
	var pks []string
	for rws.Next() {
		var pk string
		if err := rws.Scan(&pk); err != nil {
			return nil, err
		}
		pks = append(pks, pk)
	}
	return pks, rws.Err()
}
