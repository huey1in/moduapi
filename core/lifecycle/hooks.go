// Package lifecycle runs ordered, cancellable hooks with no business policies.
package lifecycle

import (
	"context"

	"github.com/huey1in/moduapi/sdk"
)

type Stage string

const (
	Ingress    Stage = "ingress"
	Decoded    Stage = "decoded"
	Routed     Stage = "routed"
	Selected   Stage = "selected"
	AttemptEnd Stage = "attempt_end"
	Complete   Stage = "complete"
)

type Event struct {
	Stage   Stage
	Request *sdk.RequestInfo
	Attempt *sdk.AttemptResult
	Usage   *sdk.UsageEvent
	Failure *sdk.Failure
}
type Hook interface {
	Handle(context.Context, Event) error
}
type HookFunc func(context.Context, Event) error

func (f HookFunc) Handle(ctx context.Context, e Event) error { return f(ctx, e) }

type Bus struct{ hooks []Hook }

func New(hooks ...Hook) *Bus { return &Bus{hooks: append([]Hook(nil), hooks...)} }
func (b *Bus) Emit(ctx context.Context, e Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b == nil {
		return nil
	}
	for _, h := range b.hooks {
		if err := h.Handle(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
