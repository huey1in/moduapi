package main

import (
	"context"
	"encoding/json"
	"fmt"
	scheduler "github.com/huey1in/moduapi/plugins/official/scheduler-priority-weight-health"
	"github.com/huey1in/moduapi/sdk"
	"github.com/huey1in/moduapi/sdk/plugin"
	"os"
)

func main() {
	var s *scheduler.Scheduler
	// Configure constructs state before admission; the SDK serializes it.
	d := plugin.Definition{Info: &sdk.Description{Id: "scheduler.priority-weight-health", Version: "0.1.0", Provides: []string{sdk.KindScheduler}, Capabilities: []string{"affinity"}},
		Configure: func(ctx context.Context, raw json.RawMessage) error {
			c, e := scheduler.ParseConfig(raw)
			if e != nil {
				return e
			}
			s, e = scheduler.New(c)
			return e
		},
		Close: func() error {
			if s != nil {
				return s.Close()
			}
			return nil
		}, Scheduler: adapter{get: func() *scheduler.Scheduler { return s }}}
	if e := plugin.Run(d); e != nil {
		fmt.Fprintln(os.Stderr, "scheduler stopped")
		os.Exit(1)
	}
}

type adapter struct{ get func() *scheduler.Scheduler }

func (a adapter) Select(ctx context.Context, in *sdk.SelectionInput) (*sdk.Selection, error) {
	return a.get().Select(ctx, in)
}
func (a adapter) Report(ctx context.Context, in *sdk.AttemptResult) error {
	return a.get().Report(ctx, in)
}
