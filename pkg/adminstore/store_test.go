package adminstore

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRootUserAndSession(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.CreateRootUser(ctx, "root", "password-ok-10"); err != nil {
		t.Fatal(err)
	}
	sid, err := s.Authenticate(ctx, "root", "password-ok-10")
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.ResolveSession(ctx, sid)
	if err != nil || !u.IsRoot {
		t.Fatalf("session: %+v err=%v", u, err)
	}
}
