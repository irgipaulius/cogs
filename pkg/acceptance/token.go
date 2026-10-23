// Package acceptance provides loopback-only tokens for automated acceptance tests.
package acceptance

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/adminstore"
)

const (
	tokenFile  = "acceptance.token"
	cookieName = "cogs_acceptance_admin"
	headerName = "X-Cogs-Acceptance-Token"
)

// TokenPath is the loopback acceptance token file under the workspace.
func TokenPath(wsRoot string) string {
	return filepath.Join(wsRoot, ".cogs", "run", tokenFile)
}

// EnsureToken creates or returns the loopback acceptance token (mode 0600).
func EnsureToken(wsRoot string) (string, error) {
	path := TokenPath(wsRoot)
	if b, err := os.ReadFile(path); err == nil {
		tok := strings.TrimSpace(string(b))
		if tok != "" {
			return tok, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(raw[:])
	if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
		return "", err
	}
	return tok, nil
}

// ReadToken returns the token if present.
func ReadToken(wsRoot string) (string, bool) {
	b, err := os.ReadFile(TokenPath(wsRoot))
	if err != nil {
		return "", false
	}
	tok := strings.TrimSpace(string(b))
	return tok, tok != ""
}

// RequestToken reads token from cookie or header.
func RequestToken(r *http.Request) string {
	if c, err := r.Cookie(cookieName); err == nil {
		return strings.TrimSpace(c.Value)
	}
	return strings.TrimSpace(r.Header.Get(headerName))
}

// IsLoopbackRequest is true for local connections only.
func IsLoopbackRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}

// RootUserIfAccepted returns a synthetic root user when loopback token matches.
func RootUserIfAccepted(wsRoot string, r *http.Request) (*adminstore.EffectiveUser, bool) {
	if !IsLoopbackRequest(r) {
		return nil, false
	}
	want, ok := ReadToken(wsRoot)
	if !ok {
		return nil, false
	}
	got := RequestToken(r)
	if got == "" || got != want {
		return nil, false
	}
	return &adminstore.EffectiveUser{IsRoot: true, Username: "acceptance"}, true
}

// CookieName is the loopback acceptance cookie name.
func CookieName() string { return cookieName }
