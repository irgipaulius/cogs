package gowork

import "testing"

func TestUsePathFromRoot(t *testing.T) {
	ws := "/workspace/cogs"
	if got := UsePathFromRoot(ws, "apps/admin"); got != "./../apps/admin" {
		t.Fatalf("admin=%q", got)
	}
	if got := UsePathFromRoot(ws, ".cogs/clones/foo"); got != "./clones/foo" {
		t.Fatalf("clone=%q", got)
	}
}

func TestNormalizeOverlayUsesIdempotent(t *testing.T) {
	ws := t.TempDir()
	uses := []string{"../apps/admin", "./clones/foo"}
	out := NormalizeOverlayUses(ws, uses)
	if len(out) != 2 || out[0] != "../apps/admin" {
		t.Fatalf("%v", out)
	}
}
