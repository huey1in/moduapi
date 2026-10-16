package gateway

import (
	"context"
	"testing"

	"github.com/huey1in/moduapi/core/retry"
)

type permissive struct{ calls int }

func (p *permissive) Decide(context.Context, retry.State) retry.Decision {
	p.calls++
	return retry.Decision{Retry: true}
}

func TestPolicyCannotRelaxKernelReplayBoundary(t *testing.T) {
	p := &permissive{}
	g := &Gateway{policy: p}
	for _, s := range []retry.State{{Attempt: 1, MaxAttempts: 2, AllowReplay: true, Committed: true}, {Attempt: 1, MaxAttempts: 2, AllowReplay: false}, {Attempt: 2, MaxAttempts: 2, AllowReplay: true}} {
		if g.retryDecision(context.Background(), s).Retry {
			t.Fatalf("unsafe replay accepted: %+v", s)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if g.retryDecision(ctx, retry.State{Attempt: 1, MaxAttempts: 2, AllowReplay: true}).Retry || p.calls != 0 {
		t.Fatal("policy bypassed cancellation")
	}
	if !g.retryDecision(context.Background(), retry.State{Attempt: 1, MaxAttempts: 2, AllowReplay: true}).Retry || p.calls != 1 {
		t.Fatal("valid policy was not consulted")
	}
}
