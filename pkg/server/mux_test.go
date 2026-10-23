package server

import (
	"net/http"
	"testing"

	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/observe"
)

func TestNewMuxGoldilocksStyleRoutes(t *testing.T) {
	obs := &observe.Runtime{Mode: observe.ModeDashboard, Hub: observe.NewHub(8)}
	mux := NewMux(Deps{
		Config:       config.App{DBPath: ":memory:"},
		MigrationVer: 1,
		Observe:      obs,
		RegisterRoutes: func(m *http.ServeMux) {
			m.Handle("GET /", http.NotFoundHandler())
			m.Handle("GET /index.html", http.NotFoundHandler())
		},
	})
	if mux == nil {
		t.Fatal("nil mux")
	}
}
