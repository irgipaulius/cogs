package reconcile

import (
	"bytes"
	"context"
	"testing"

	"github.com/irgipaulius/cogs/pkg/state"
)

func TestStageRecordsLastError(t *testing.T) {
	root := t.TempDir()
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetDesired(state.AppDesired{ID: "todo", Present: true}); err != nil {
		t.Fatal(err)
	}
	runID, err := st.BeginRun("todo", "")
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	err = stage(st, "todo", runID, "cf_resources", &log, func() error {
		return errAuth
	})
	if err == nil {
		t.Fatal("expected error")
	}
	app, _ := st.GetApp("todo")
	if app.LastError == "" {
		t.Fatal("expected last_error on stage failure")
	}
	_ = context.Background()
}

var errAuth = &stageErr{msg: "Authentication error [code: 10000]"}

type stageErr struct{ msg string }

func (e *stageErr) Error() string { return e.msg }
