// Package adminstore is the SQLite backing store for the public admin dashboard.
package adminstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/irgipaulius/cogs/pkg/sqliteopen"
)

// Permission names for RBAC grants.
const (
	PermMetricsRead = "metrics.read"
	PermLogsRead    = "logs.read"
	PermSQLRead     = "sql.read"
	PermSQLWrite    = "sql.write"
	PermRedeploy    = "redeploy"
	PermUsersManage = "users.manage"
)

var AllPermissions = []string{
	PermMetricsRead, PermLogsRead, PermSQLRead, PermSQLWrite, PermRedeploy, PermUsersManage,
}

// Store wraps admin.db.
type Store struct {
	db *sql.DB
}

// Open opens or creates admin.db and runs migrations.
func Open(path string) (*Store, error) {
	pools, err := sqliteopen.Open(sqliteopen.Config{Path: path, ReaderConns: 2})
	if err != nil {
		return nil, err
	}
	s := &Store{db: pools.Writer}
	if err := migrate(s.db); err != nil {
		pools.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB { return s.db }

// HasAnyUser reports whether at least one user exists (launch gate).
func (s *Store) HasAnyUser(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n > 0, err
}

// User row.
type User struct {
	ID           int64
	Username     string
	IsRoot       bool
	PasswordHash string
	CreatedAt    time.Time
}

// Session row.
type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
}

// Grant ties a user to an app and permission.
type Grant struct {
	ID         int64
	UserID     int64
	AppName    string
	Permission string
}

// AuditLog entry.
type AuditLog struct {
	ID        int64
	UserID    int64
	Username  string
	Action    string
	Detail    string
	CreatedAt time.Time
}

// EffectiveUser is the authenticated principal.
type EffectiveUser struct {
	ID       int64
	Username string
	IsRoot   bool
	Grants   []Grant
}

func (u *EffectiveUser) HasPermission(appName, perm string) bool {
	if u.IsRoot {
		return true
	}
	for _, g := range u.Grants {
		if g.Permission != perm {
			continue
		}
		if g.AppName == "*" || g.AppName == appName {
			return true
		}
	}
	return false
}

func (u *EffectiveUser) CanGrant(perm string) bool {
	if u.IsRoot {
		return true
	}
	return u.HasPermission("*", perm)
}

// ResolveSession loads user + grants for a session id.
func (s *Store) ResolveSession(ctx context.Context, sessionID string) (*EffectiveUser, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("adminstore: missing session")
	}
	var sess Session
	var username string
	var isRoot int
	var expiresStr string
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.expires_at, u.username, u.is_root
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = ? AND s.expires_at > ?
	`, sessionID, now).Scan(&sess.ID, &sess.UserID, &expiresStr, &username, &isRoot)
	if err == nil {
		sess.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr)
	}
	if err != nil {
		return nil, fmt.Errorf("adminstore: invalid session")
	}
	grants, err := s.listGrants(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	return &EffectiveUser{
		ID:       sess.UserID,
		Username: username,
		IsRoot:   isRoot != 0,
		Grants:   grants,
	}, nil
}

func (s *Store) listGrants(ctx context.Context, userID int64) ([]Grant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, app_name, permission FROM grants WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Grant
	for rows.Next() {
		var g Grant
		if err := rows.Scan(&g.ID, &g.UserID, &g.AppName, &g.Permission); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
