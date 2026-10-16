package router

import (
	"context"
	"github.com/huey1in/moduapi/sdk"
	"testing"
)

func TestRoutingAndCancellation(t *testing.T) {
	routes := []Route{{ID: "exact", Protocol: "a.chat", Model: "m", Upstreams: []string{"a", "b"}, Scheduler: "s", Retry: Retry{MaxAttempts: 1}}, {ID: "wild", Protocol: "a.chat", Model: "*", Upstreams: []string{"b"}, Scheduler: "s", Retry: Retry{MaxAttempts: 1}}}
	r, e := New(routes)
	if e != nil {
		t.Fatal(e)
	}
	routes[0].Upstreams[0] = "mutated"
	for model, want := range map[string]string{"m": "exact", "other": "wild"} {
		got, e := r.Match(context.Background(), &sdk.Request{Protocol: "a.chat", Model: model})
		if e != nil || got.ID != want {
			t.Fatalf("%s: %v %v", model, got, e)
		}
		if got.Upstreams[0] == "mutated" {
			t.Fatal("router retained mutable configuration")
		}
	}
	if _, e = r.Match(context.Background(), &sdk.Request{Protocol: "unknown", Model: "m"}); e == nil {
		t.Fatal("expected route miss")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = r.Match(ctx, &sdk.Request{Protocol: "a.chat", Model: "m"}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestRouteValidation(t *testing.T) {
	for _, r := range []Route{{}, {ID: "x", Protocol: "p", Model: "m*", Upstreams: []string{"a"}, Scheduler: "s", Retry: Retry{MaxAttempts: 1}}, {ID: "x", Protocol: "p", Model: "*", Upstreams: []string{"a", "a"}, Scheduler: "s", Retry: Retry{MaxAttempts: 1}}, {ID: "x", Protocol: "p", Model: "*", Upstreams: []string{"a"}, Scheduler: "s", Retry: Retry{MaxAttempts: 9}}} {
		if _, e := New([]Route{r}); e == nil {
			t.Fatalf("accepted invalid route %+v", r)
		}
	}
}
