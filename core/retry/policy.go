// Package retry decides replay independently from selection and HTTP execution.
package retry

import (
	"context"
	"time"

	"github.com/huey1in/moduapi/sdk"
)

type State struct {
	Attempt, MaxAttempts   int
	AllowReplay, Committed bool
	Failure                *sdk.Failure
}
type Decision struct {
	Retry  bool
	Reason string
	Delay  time.Duration
}
type Policy interface {
	Decide(context.Context, State) Decision
}
type Conservative struct{}

func (Conservative) Decide(ctx context.Context, s State) Decision {
	if ctx.Err() != nil || s.Committed || !s.AllowReplay || s.Attempt >= s.MaxAttempts || s.Failure == nil {
		return Decision{Reason: "replay prohibited or budget exhausted"}
	}
	f := s.Failure
	allowed := f.Kind == sdk.Transport || f.Kind == sdk.Timeout || (f.Kind == sdk.HTTP && (f.Status == 429 || f.Status == 502 || f.Status == 503 || f.Status == 504))
	if !allowed {
		return Decision{Reason: "non-retryable failure"}
	}
	return Decision{Retry: true, Reason: "pre-commit transient failure", Delay: time.Duration(s.Attempt-1) * 25 * time.Millisecond}
}
