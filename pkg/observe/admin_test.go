package observe

import (
	"net/http"
	"testing"
)

func TestRegisterAdminRoutesWithMethodRoot(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := &Runtime{Mode: ModeDashboard, Hub: NewHub(64)}
	r.RegisterAdminRoutes(mux, "", nil)
}
