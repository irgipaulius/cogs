package limiter

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

func TestLRUEviction(t *testing.T) {
	l := NewLRU(2, rate.Every(time.Second), 1)
	c := &fakeClock{t: time.Unix(0, 0)}
	l.SetClock(c)

	if !l.Allow("a") {
		t.Fatal("expected allow a")
	}
	if !l.Allow("b") {
		t.Fatal("expected allow b")
	}
	if !l.Allow("c") {
		t.Fatal("expected allow c")
	}
	// "a" should be evicted; fresh limiter for "a"
	if !l.Allow("a") {
		t.Fatal("expected allow a after eviction")
	}
}
