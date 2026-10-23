package state

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

const dbFile = "state.db"

// Store is the workspace lifecycle database.
type Store struct {
	wsRoot string
	db     *sql.DB
	mu     sync.RWMutex

	subsMu sync.RWMutex
	subs   map[chan struct{}]struct{}
}

// Open opens or creates .cogs/state.db and runs migrations + legacy import once.
func Open(wsRoot string) (*Store, error) {
	wsRoot, err := filepath.Abs(wsRoot)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(wsRoot, ".cogs"), 0o750); err != nil {
		return nil, err
	}
	path := filepath.Join(wsRoot, ".cogs", dbFile)
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	st := &Store{wsRoot: wsRoot, db: db, subs: map[chan struct{}]struct{}{}}
	if err := st.applySchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := importLegacyOnce(st); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) WSRoot() string { return s.wsRoot }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) applySchema() error {
	b, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	for _, stmt := range splitSQL(string(b)) {
		if stmt == "" {
			continue
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("schema: %w (stmt: %.80s)", err, stmt)
		}
	}
	var v int
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&v)
	if v == 0 {
		_, err = s.db.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (1, ?)`, time.Now().UTC().Format(time.RFC3339))
	}
	return err
}

func splitSQL(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ";") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) notify() {
	s.subsMu.RLock()
	defer s.subsMu.RUnlock()
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Subscribe returns a channel notified on store changes.
func (s *Store) Subscribe() chan struct{} {
	ch := make(chan struct{}, 8)
	s.subsMu.Lock()
	s.subs[ch] = struct{}{}
	s.subsMu.Unlock()
	return ch
}

// Unsubscribe removes a subscription channel.
func (s *Store) Unsubscribe(ch chan struct{}) {
	s.subsMu.Lock()
	delete(s.subs, ch)
	s.subsMu.Unlock()
}

func (s *Store) tx(fn func(*sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.notify()
	return nil
}
