package gowork

import "testing"

func TestIsHermeticBuildDir(t *testing.T) {
	ws := "/data/cogs"
	wt := ws + "/.cogs/worktrees/todo/abc123"
	if !IsHermeticBuildDir(ws, wt) {
		t.Fatal("worktree should be hermetic")
	}
	app := ws + "/apps/admin"
	if IsHermeticBuildDir(ws, app) {
		t.Fatal("workspace app should use overlay")
	}
}
