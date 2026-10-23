package state

import "testing"

func TestKeepReleaseSHAsRetainsOnlyLimitPlusPointers(t *testing.T) {
	root := t.TempDir()
	st, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	app := "todo"
	if err := st.SetDesired(AppDesired{ID: app, Present: true}); err != nil {
		t.Fatal(err)
	}
	for i, sha := range []string{"aaaa", "bbbb", "cccc", "dddd", "eeee"} {
		if _, err := st.UpsertRelease(app, sha, "/bin/"+sha, true); err != nil {
			t.Fatal(err)
		}
		if i == 4 {
			id, _ := st.UpsertRelease(app, sha, "/bin/"+sha, true)
			_ = st.ActivateRelease(app, id)
		}
	}
	keep, err := st.KeepReleaseSHAs(app, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) > 5 {
		t.Fatalf("keep too large: %v", keep)
	}
}
