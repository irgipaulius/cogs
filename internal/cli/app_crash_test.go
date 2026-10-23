package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/state"
)

func TestTailAppLogPanic(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, ".cogs", "run")
	_ = os.MkdirAll(run, 0o755)
	_ = os.WriteFile(filepath.Join(run, "app.log"), []byte("ok\npanic: route conflict\n"), 0o644)
	msg := tailAppLogPanic(dir)
	if msg == "" || !containsSub(msg, "panic") {
		t.Fatalf("got %q", msg)
	}
}

func TestRecordAppCrashIfAnySetsLastError(t *testing.T) {
	root := t.TempDir()
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.SetDesired(state.AppDesired{ID: "goldilocks", Present: true})
	dir := filepath.Join(root, "apps", "goldilocks")
	run := filepath.Join(dir, ".cogs", "run")
	_ = os.MkdirAll(run, 0o755)
	_ = os.WriteFile(filepath.Join(run, "app.log"), []byte("panic: test\n"), 0o644)
	recordAppCrashIfAny(st, "goldilocks", dir)
	app, _ := st.GetApp("goldilocks")
	if app.LastError == "" {
		t.Fatal("expected last_error")
	}
}

func containsSub(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexSub(s, sub))
}

func indexSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
