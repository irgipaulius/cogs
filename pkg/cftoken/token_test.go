package cftoken

import (
	"context"
	"testing"
)

func TestQueueScopePrefersQueueToken(t *testing.T) {
	t.Setenv("COGS_CF_QUEUE_API_TOKEN", "queue-tok")
	t.Setenv("COGS_CF_API_TOKEN", "api-tok")
	r := Resolver{}
	got, err := r.For(context.Background(), ScopeQueue)
	if err != nil {
		t.Fatal(err)
	}
	if got != "queue-tok" {
		t.Fatalf("got %q want queue-tok", got)
	}
}
