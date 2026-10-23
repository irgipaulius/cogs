package state

import (
	"path/filepath"
	"testing"
)

func TestSetConvergedOnlyPreservesLastError(t *testing.T) {
	root := t.TempDir()
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetDesired(AppDesired{ID: "todo", Present: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetConverged("todo", false, "cf_resources: auth"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetConvergedOnly("todo", false); err != nil {
		t.Fatal(err)
	}
	app, err := st.GetApp("todo")
	if err != nil {
		t.Fatal(err)
	}
	if app.LastError != "cf_resources: auth" {
		t.Fatalf("last_error=%q want preserved", app.LastError)
	}
	_ = filepath.Join(root, ".cogs")
}
