package session

import (
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	m := Manager{Secret: []byte("test-secret"), TTL: time.Minute}
	now := time.Unix(1000, 0)
	tok, err := m.Mint("1.2.3.4", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(tok, "1.2.3.4", now); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestSessionExpired(t *testing.T) {
	m := Manager{Secret: []byte("test-secret"), TTL: time.Second}
	now := time.Unix(1000, 0)
	tok, _ := m.Mint("1.2.3.4", now)
	if err := m.Validate(tok, "1.2.3.4", now.Add(2*time.Second)); err == nil {
		t.Fatal("expected expired")
	}
}

func TestSessionWrongIP(t *testing.T) {
	m := Manager{Secret: []byte("test-secret"), TTL: time.Minute}
	now := time.Unix(1000, 0)
	tok, _ := m.Mint("1.2.3.4", now)
	if err := m.Validate(tok, "5.6.7.8", now); err == nil {
		t.Fatal("expected wrong ip")
	}
}

func TestSessionTampered(t *testing.T) {
	m := Manager{Secret: []byte("test-secret"), TTL: time.Minute}
	now := time.Unix(1000, 0)
	tok, _ := m.Mint("1.2.3.4", now)
	tok = tok[:len(tok)-2] + "xx"
	if err := m.Validate(tok, "1.2.3.4", now); err == nil {
		t.Fatal("expected tampered")
	}
}
