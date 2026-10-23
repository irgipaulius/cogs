package cfprovision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/irgipaulius/cogs/pkg/cfclient"
)

func TestEnsureD1DatabaseUsesAPIWithoutWrangler(t *testing.T) {
	var posts int
	var have bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && stringsHasSuffix(r.URL.Path, "/d1/database"):
			res := []any{}
			if have {
				res = []any{map[string]any{"uuid": "u1", "name": "cogs-todo-edge"}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  res,
			})
		case r.Method == http.MethodPost && stringsHasSuffix(r.URL.Path, "/d1/database"):
			posts++
			have = true
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]any{"uuid": "u1"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &cfclient.Client{TestBase: srv.URL, TestToken: "test-token"}
	ctx := context.Background()
	if err := ensureD1Database(ctx, c, "acc", "cogs-todo-edge"); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatalf("expected 1 create POST, got %d", posts)
	}
	if err := ensureD1Database(ctx, c, "acc", "cogs-todo-edge"); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatalf("expected no second POST after existing db, got %d", posts)
	}
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
