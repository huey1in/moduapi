// Package roundrobin provides a process-local, per-route scheduler for this slice.
package roundrobin

import (
	"context"
	"github.com/huey1in/moduapi/sdk"
	"slices"
	"sync"
)

type Scheduler struct {
	mu   sync.Mutex
	next map[string]uint64
}

func (s *Scheduler) Select(ctx context.Context, in *sdk.SelectionInput) (*sdk.Selection, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	var eligible []*sdk.Candidate
	for _, c := range in.Candidates {
		if !slices.Contains(in.ExcludedIds, c.Id) {
			eligible = append(eligible, c)
		}
	}
	if len(eligible) == 0 {
		return nil, sdk.Fail(sdk.Local, "no_candidate", "no eligible upstream", 503)
	}
	s.mu.Lock()
	if s.next == nil {
		s.next = map[string]uint64{}
	}
	n := s.next[in.RouteId]
	s.next[in.RouteId] = n + 1
	s.mu.Unlock()
	return &sdk.Selection{CandidateId: eligible[n%uint64(len(eligible))].Id, Reason: "round robin"}, nil
}
func (*Scheduler) Report(ctx context.Context, _ *sdk.AttemptResult) error { return ctx.Err() }
