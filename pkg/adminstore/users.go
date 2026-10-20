package adminstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 2
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
	sessionTTL   = 7 * 24 * time.Hour
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		hex.EncodeToString(salt), hex.EncodeToString(key)), nil
}

func verifyPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) < 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	saltHex, keyHex := parts[4], parts[5]
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(keyHex)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtleConstantTimeCompare(got, want)
}

func subtleConstantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// CreateRootUser creates the first root account (admin init).
func (s *Store) CreateRootUser(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if len(username) < 2 {
		return fmt.Errorf("username too short")
	}
	if len(password) < 10 {
		return fmt.Errorf("password must be at least 10 characters")
	}
	ok, err := s.HasAnyUser(ctx)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("admin already initialized")
	}
	h, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, is_root) VALUES (?, ?, 1)
	`, username, h)
	return err
}

// Authenticate verifies credentials and creates a session.
func (s *Store) Authenticate(ctx context.Context, username, password string) (sessionID string, err error) {
	var id int64
	var hash string
	err = s.db.QueryRowContext(ctx, `
		SELECT id, password_hash FROM users WHERE username = ? COLLATE NOCASE
	`, strings.TrimSpace(username)).Scan(&id, &hash)
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	if !verifyPassword(hash, password) {
		return "", fmt.Errorf("invalid credentials")
	}
	sid, err := newSessionID()
	if err != nil {
		return "", err
	}
	exp := time.Now().UTC().Add(sessionTTL)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)
	`, sid, id, exp.Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	return sid, nil
}

func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Logout removes a session.
func (s *Store) Logout(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

// ListUsers returns non-sensitive user list.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, is_root, created_at FROM users ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var isRoot int
		var created string
		if err := rows.Scan(&u.ID, &u.Username, &isRoot, &created); err != nil {
			return nil, err
		}
		u.IsRoot = isRoot != 0
		u.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, u)
	}
	return out, rows.Err()
}

// CreateUser adds a non-root user with password.
func (s *Store) CreateUser(ctx context.Context, username, password string) (int64, error) {
	username = strings.TrimSpace(username)
	if len(username) < 2 {
		return 0, fmt.Errorf("username too short")
	}
	if len(password) < 10 {
		return 0, fmt.Errorf("password must be at least 10 characters")
	}
	h, err := hashPassword(password)
	if err != nil {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, is_root) VALUES (?, ?, 0)
	`, username, h)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetGrants replaces grants for a user (non-root only).
func (s *Store) SetGrants(ctx context.Context, userID int64, grants []Grant) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM grants WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, g := range grants {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO grants (user_id, app_name, permission) VALUES (?, ?, ?)
		`, userID, g.AppName, g.Permission); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListGrantsForUser returns grants for one user.
func (s *Store) ListGrantsForUser(ctx context.Context, userID int64) ([]Grant, error) {
	return s.listGrants(ctx, userID)
}

// WriteAudit appends an audit row.
func (s *Store) WriteAudit(ctx context.Context, user *EffectiveUser, action, detail string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (user_id, username, action, detail) VALUES (?, ?, ?, ?)
	`, user.ID, user.Username, action, detail)
	return err
}

// ListAudit returns recent audit entries.
func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, username, action, detail, created_at
		FROM audit_log ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var a AuditLog
		var created string
		if err := rows.Scan(&a.ID, &a.UserID, &a.Username, &a.Action, &a.Detail, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, a)
	}
	return out, rows.Err()
}
