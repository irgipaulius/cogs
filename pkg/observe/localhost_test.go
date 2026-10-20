package observe

import (
	"net/http/httptest"
	"testing"
)

func TestLocalAdminHost(t *testing.T) {
	r := httptest.NewRequest("GET", "http://127.0.0.1:8080/admin", nil)
	if !LocalAdminHost(r) {
		t.Fatal("expected loopback")
	}
	r = httptest.NewRequest("GET", "http://todo-api.example.com/admin", nil)
	if LocalAdminHost(r) {
		t.Fatal("expected public host blocked")
	}
}
