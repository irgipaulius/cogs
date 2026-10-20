package todo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Item struct {
	ID         string `json:"id"`
	ClientID   string `json:"client_id"`
	Title      string `json:"title"`
	Done       bool   `json:"done"`
	ImageR2Key string `json:"image_r2_key,omitempty"`
	UpdatedAt  string `json:"updated_at"`
}

// SQLHandle is satisfied by *sql.DB and observe.InstrumentedDB.
type SQLHandle interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type Store struct {
	DB SQLHandle
}

func (s *Store) List(ctx context.Context, since, clientID string) ([]Item, error) {
	var rows *sql.Rows
	var err error
	if clientID != "" {
		if since != "" {
			rows, err = s.DB.QueryContext(ctx, `
				SELECT id, client_id, title, done, COALESCE(image_r2_key,''), updated_at
				FROM todos WHERE deleted=0 AND client_id=? AND updated_at > ? ORDER BY updated_at DESC`,
				clientID, since)
		} else {
			rows, err = s.DB.QueryContext(ctx, `
				SELECT id, client_id, title, done, COALESCE(image_r2_key,''), updated_at
				FROM todos WHERE deleted=0 AND client_id=? ORDER BY updated_at DESC LIMIT 500`, clientID)
		}
	} else if since != "" {
		rows, err = s.DB.QueryContext(ctx, `
			SELECT id, client_id, title, done, COALESCE(image_r2_key,''), updated_at
			FROM todos WHERE deleted=0 AND updated_at > ? ORDER BY updated_at DESC`, since)
	} else {
		rows, err = s.DB.QueryContext(ctx, `
			SELECT id, client_id, title, done, COALESCE(image_r2_key,''), updated_at
			FROM todos WHERE deleted=0 ORDER BY updated_at DESC LIMIT 500`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		var done int
		if err := rows.Scan(&it.ID, &it.ClientID, &it.Title, &done, &it.ImageR2Key, &it.UpdatedAt); err != nil {
			return nil, err
		}
		it.Done = done == 1
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) Upsert(ctx context.Context, it Item) error {
	if it.ID == "" || it.ClientID == "" {
		return errors.New("id and client_id required")
	}
	if it.UpdatedAt == "" {
		it.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	done := 0
	if it.Done {
		done = 1
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO todos (id, client_id, title, done, image_r2_key, updated_at, deleted)
		VALUES (?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title,
			done=excluded.done,
			image_r2_key=COALESCE(excluded.image_r2_key, todos.image_r2_key),
			updated_at=excluded.updated_at,
			deleted=0
		WHERE excluded.updated_at >= todos.updated_at`,
		it.ID, it.ClientID, it.Title, done, nullStr(it.ImageR2Key), it.UpdatedAt)
	return err
}

func (s *Store) SoftDelete(ctx context.Context, id, clientID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.DB.ExecContext(ctx, `
		UPDATE todos SET deleted=1, updated_at=? WHERE id=? AND client_id=?`, now, id, clientID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ImageKeyByID returns stored blob key for a todo (any client). Used for GET /image via <img src>, which cannot send X-Client-Id.
func (s *Store) ImageKeyByID(ctx context.Context, id string) (string, error) {
	var key string
	err := s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(image_r2_key,'') FROM todos WHERE id=? AND deleted=0`, id).Scan(&key)
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", sql.ErrNoRows
	}
	return key, nil
}

func (s *Store) Get(ctx context.Context, id, clientID string) (Item, error) {
	var it Item
	var done int
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, client_id, title, done, COALESCE(image_r2_key,''), updated_at
		FROM todos WHERE id=? AND client_id=? AND deleted=0`, id, clientID).Scan(
		&it.ID, &it.ClientID, &it.Title, &done, &it.ImageR2Key, &it.UpdatedAt)
	if err != nil {
		return Item{}, err
	}
	it.Done = done == 1
	return it, nil
}

func (s *Store) SetImageKey(ctx context.Context, id, clientID, key string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.DB.ExecContext(ctx, `
		UPDATE todos SET image_r2_key=?, updated_at=? WHERE id=? AND client_id=? AND deleted=0`,
		key, now, id, clientID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) Seed(ctx context.Context, n int, clientID string) error {
	base := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("seed-%s-%d", base, i)
		it := Item{ID: id, ClientID: clientID, Title: "seed " + id, UpdatedAt: base}
		if err := s.Upsert(ctx, it); err != nil {
			return err
		}
	}
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
