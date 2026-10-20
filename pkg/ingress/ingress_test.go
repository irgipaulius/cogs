package ingress

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareTunnelOnlyBlocksDirect(t *testing.T) {
	cfg := &Config{TrustLoopback: false, Routes: []Route{
		{Pattern: "GET /api/x", AllowDirect: false},
	}}
	next := httptest.NewRecorder()
	h := Middleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "http://example.com/api/x", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code=%d", rec.Code)
	}
	_ = next
}

func TestMiddlewareAllowDirectRoute(t *testing.T) {
	cfg := &Config{TrustLoopback: false, Routes: []Route{
		{Pattern: "POST /api/bulk/*", AllowDirect: true},
	}}
	h := Middleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	req := httptest.NewRequest("POST", "http://example.com/api/bulk/upload", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestViaCloudflare(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cf-Ray", "abc")
	if !ViaCloudflare(req) {
		t.Fatal("expected cf")
	}
}
