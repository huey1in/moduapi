// Package single chooses the first eligible candidate, including failover exclusions.
package single

import (
	"context"
	"github.com/huey1in/moduapi/sdk"
	"slices"
)

type Scheduler struct{}

func (Scheduler) Select(ctx context.Context, in *sdk.SelectionInput) (*sdk.Selection, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	for _, c := range in.Candidates {
		if !slices.Contains(in.ExcludedIds, c.Id) {
			return &sdk.Selection{CandidateId: c.Id, Reason: "first eligible candidate"}, nil
		}
	}
	return nil, sdk.Fail(sdk.Local, "no_candidate", "no eligible upstream", 503)
}
func (Scheduler) Report(ctx context.Context, _ *sdk.AttemptResult) error { return ctx.Err() }
