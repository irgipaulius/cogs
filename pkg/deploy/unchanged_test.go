package deploy

import "testing"

func TestShasEqualMatchesPrefix(t *testing.T) {
	if !shasEqual("1268cb7abc", "1268cb7") {
		t.Fatal("expected prefix match for skip-unchanged deploy")
	}
}
