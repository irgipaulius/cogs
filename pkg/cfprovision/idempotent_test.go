package cfprovision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/irgipaulius/cogs/pkg/cfclient"
)

func TestFindD1DoesNotPostWhenDatabaseExists(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  []any{map[string]any{"uuid": "id1", "name": "cogs-todo-edge"}},
			})
			return
		}
		if r.Method == http.MethodPost {
			posts++
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]any{}})
	}))
	defer srv.Close()
	c := &cfclient.Client{TestBase: srv.URL, TestToken: "t"}
	if err := ensureD1Database(context.Background(), c, "acc", "cogs-todo-edge"); err != nil {
		t.Fatal(err)
	}
	if posts != 0 {
		t.Fatalf("expected 0 POST create, got %d", posts)
	}
}
