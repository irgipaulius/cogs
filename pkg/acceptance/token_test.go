package acceptance

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRootUserIfAcceptedLoopbackOnly(t *testing.T) {
	root := t.TempDir()
	tok, err := EnsureToken(root)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://127.0.0.1/api/me", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	if _, ok := RootUserIfAccepted(root, req); !ok {
		t.Fatal("expected acceptance root on loopback")
	}
	req2 := httptest.NewRequest("GET", "http://evil.example/api/me", nil)
	req2.RemoteAddr = "203.0.113.1:1234"
	req2.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	if _, ok := RootUserIfAccepted(root, req2); ok {
		t.Fatal("must not accept token off loopback")
	}
}
