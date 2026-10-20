package cockpit

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPCloudflare(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("CF-Connecting-IP", "203.0.113.10")
	r.RemoteAddr = "172.19.0.4:54321"
	if got := ClientIP(r); got != "203.0.113.10" {
		t.Fatalf("got %q", got)
	}
}

func TestClientCountry(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("CF-IPCountry", "lt")
	if got := ClientCountry(r); got != "LT" {
		t.Fatalf("got %q", got)
	}
}
