package supervisor

import "testing"

func TestIsReadOnlySQL(t *testing.T) {
	if !isReadOnlySQL("WITH x AS (SELECT 1) SELECT * FROM x") {
		t.Fatal("expected WITH to be read-only")
	}
	if isReadOnlySQL("DELETE FROM todos") {
		t.Fatal("DELETE should not be read-only")
	}
}
