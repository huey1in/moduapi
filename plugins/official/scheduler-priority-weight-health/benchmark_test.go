package scheduler

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/huey1in/moduapi/sdk"
)

func benchmarkFixture(b *testing.B, n int) (*Scheduler, *sdk.SelectionInput) {
	b.Helper()
	s, e := New(Defaults())
	if e != nil {
		b.Fatal(e)
	}
	b.Cleanup(func() { s.Close() })
	in := input()
	for i := range n {
		in.Candidates = append(in.Candidates, candidate(fmt.Sprint(i), "10", "2", ""))
	}
	return s, in
}
func releaseBenchmark(s *Scheduler, v *sdk.Selection) {
	_ = s.Report(context.Background(), &sdk.AttemptResult{RouteId: "route", CandidateId: v.CandidateId, Lease: v.Lease, Status: 200})
}
func BenchmarkSchedulerSelect(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s, in := benchmarkFixture(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; {
				batch := min(256, b.N-i)
				selections := make([]*sdk.Selection, 0, batch)
				for range batch {
					v, e := s.Select(context.Background(), in)
					if e != nil {
						b.Fatal(e)
					}
					selections = append(selections, v)
				}
				b.StopTimer()
				for _, v := range selections {
					releaseBenchmark(s, v)
				}
				b.StartTimer()
				i += batch
			}
		})
	}
}

// Parallel admission is measured as Select+Report cycles so the test does not
// turn into a measurement of the configured lease table limit.
func BenchmarkSchedulerSelectParallel(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		for _, workers := range []int{1, 10, 100, 1000} {
			b.Run(fmt.Sprintf("candidates=%d/workers=%d", n, workers), func(b *testing.B) {
				s, in := benchmarkFixture(b, n)
				var next atomic.Int64
				var wg sync.WaitGroup
				start := make(chan struct{})
				for range workers {
					wg.Go(func() {
						<-start
						for {
							if next.Add(1) > int64(b.N) {
								return
							}
							v, e := s.Select(context.Background(), in)
							if e != nil {
								b.Error(e)
								return
							}
							releaseBenchmark(s, v)
						}
					})
				}
				b.ReportAllocs()
				b.ResetTimer()
				close(start)
				wg.Wait()
				b.StopTimer()
			})
		}
	}
}
func BenchmarkSchedulerReport(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s, in := benchmarkFixture(b, n)
			selections := make([]*sdk.Selection, 256)
			saved := make([]lease, 256)
			for i := range selections {
				v, e := s.Select(context.Background(), in)
				if e != nil {
					b.Fatal(e)
				}
				selections[i] = v
				saved[i] = s.leases[string(v.Lease)]
				releaseBenchmark(s, v)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; {
				batch := min(256, b.N-i)
				b.StopTimer()
				// Rearm real opaque leases outside timing to isolate Report from Select cost.
				s.mu.Lock()
				for j := range batch {
					v, l := selections[j], saved[j]
					s.leases[string(v.Lease)] = l
					s.states[v.CandidateId].reserved++
				}
				s.mu.Unlock()
				b.StartTimer()
				for _, v := range selections[:batch] {
					releaseBenchmark(s, v)
				}
				i += batch
			}
		})
	}
}
func BenchmarkWeightedSelection(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			pool := make([]*choice, n)
			for i := range pool {
				pool[i] = &choice{weight: 2, factor: .8}
			}
			rng := rand.New(rand.NewPCG(1, 2))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = weighted(pool, false, rng)
			}
		})
	}
}
func BenchmarkStickySelection(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s, in := benchmarkFixture(b, n)
			affinity(in, "synthetic-benchmark-session")
			v, e := s.Select(context.Background(), in)
			if e != nil {
				b.Fatal(e)
			}
			releaseBenchmark(s, v)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				v, e := s.Select(context.Background(), in)
				if e != nil {
					b.Fatal(e)
				}
				releaseBenchmark(s, v)
			}
		})
	}
}
