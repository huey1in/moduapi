package retry

import (
	"context"
	"github.com/huey1in/moduapi/sdk"
	"testing"
)

func TestConservativeReplayBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, kind       string
		status           int32
		allow, committed bool
		attempt          int
		want             bool
	}{
		{"transport", sdk.Transport, 502, true, false, 1, true}, {"no opt-in", sdk.Transport, 502, false, false, 1, false}, {"committed", sdk.Transport, 502, true, true, 1, false}, {"budget", sdk.Transport, 502, true, false, 2, false}, {"429", sdk.HTTP, 429, true, false, 1, true}, {"503", sdk.HTTP, 503, true, false, 1, true}, {"400", sdk.HTTP, 400, true, false, 1, false}, {"401", sdk.HTTP, 401, true, false, 1, false}, {"protocol", sdk.ProtocolError, 502, true, false, 1, false}, {"local", sdk.Local, 500, true, false, 1, false}, {"downstream", sdk.Downstream, 499, true, false, 1, false}} {
		t.Run(tc.name, func(t *testing.T) {
			d := (Conservative{}).Decide(context.Background(), State{Attempt: tc.attempt, MaxAttempts: 2, AllowReplay: tc.allow, Committed: tc.committed, Failure: &sdk.Failure{Kind: tc.kind, Status: tc.status}})
			if d.Retry != tc.want {
				t.Fatalf("%+v", d)
			}
		})
	}
	ctx, c := context.WithCancel(context.Background())
	c()
	if (Conservative{}).Decide(ctx, State{Attempt: 1, MaxAttempts: 2, AllowReplay: true, Failure: &sdk.Failure{Kind: sdk.Transport}}).Retry {
		t.Fatal("cancelled request retried")
	}
}
