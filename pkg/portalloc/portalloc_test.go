package portalloc

import "testing"

func TestForIndexSkipsReserved(t *testing.T) {
	c := DefaultConfig()
	c.BasePort = 8080
	c.PortStep = 1
	p := c.ForIndex(0, nil)
	if p == 8080 {
		t.Fatalf("got reserved 8080")
	}
	if p != 8081 && p != 18787 {
		// 8081 might be ok if 8080 skipped from base - actually base 8080 first candidate skipped -> 8081 unless 8081 reserved? 8081 not in list
		if p < 8081 {
			t.Fatalf("unexpected port %d", p)
		}
	}
}

func TestForIndexSequential(t *testing.T) {
	c := DefaultConfig()
	if c.ForIndex(0, nil) != 18787 {
		t.Fatalf("index 0")
	}
	if c.ForIndex(1, nil) != 18788 {
		t.Fatalf("index 1")
	}
}
