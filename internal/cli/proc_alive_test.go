//go:build !windows

package cli

import (
	"os"
	"testing"
)

func TestProcessAliveCurrentPID(t *testing.T) {
	pid := os.Getpid()
	if !processAlive(pid) {
		t.Fatalf("expected current pid %d alive", pid)
	}
}

func TestProcessAliveMissingPID(t *testing.T) {
	// Very large pid unlikely to exist.
	if processAlive(999999999) {
		t.Fatal("expected nonexistent pid dead")
	}
}
