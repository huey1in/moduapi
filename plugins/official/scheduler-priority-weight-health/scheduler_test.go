package scheduler

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huey1in/moduapi/sdk"
)

var background = context.Background()

func fixture(t *testing.T, edit func(*Config)) (*Scheduler, *atomic.Int64) {
	t.Helper()
	c := Defaults()
	c.CleanupInterval = Duration(30 * time.Second)
	if edit != nil {
		edit(&c)
	}
	s, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	clock := &atomic.Int64{}
	clock.Store(time.Now().UnixNano())
	s.mu.Lock()
	s.now = func() time.Time { return time.Unix(0, clock.Load()) }
	s.rng = rand.New(rand.NewPCG(42, 7))
	s.mu.Unlock()
	t.Cleanup(func() { s.Close() })
	return s, clock
}
func candidate(id string, p, w, cap string) *sdk.Candidate {
	m := map[string]string{}
	for k, v := range map[string]string{sdk.SchedulerPriority: p, sdk.SchedulerWeight: w, sdk.SchedulerCapacity: cap} {
		if v != "" {
			m[k] = v
		}
	}
	return &sdk.Candidate{Id: id, Metadata: m}
}
func input(c ...*sdk.Candidate) *sdk.SelectionInput {
	return &sdk.SelectionInput{RouteId: "route", Request: &sdk.RequestInfo{Id: "r"}, Candidates: c}
}
func selectOne(t *testing.T, s *Scheduler, in *sdk.SelectionInput) *sdk.Selection {
	t.Helper()
	v, e := s.Select(background, in)
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Lease) != 16 || v.Reason == "" {
		t.Fatal(v)
	}
	return v
}
func report(t *testing.T, s *Scheduler, v *sdk.Selection, fail *sdk.Failure) {
	t.Helper()
	if e := s.Report(background, &sdk.AttemptResult{RouteId: "route", CandidateId: v.CandidateId, Lease: v.Lease, Status: 200, FirstByteMs: 10, DurationMs: 20, BytesEmitted: 30, Failure: fail}); e != nil {
		t.Fatal(e)
	}
}
func affinity(in *sdk.SelectionInput, key string) {
	in.Request.Metadata = map[string]string{sdk.AffinityKey: key}
}

func TestPriorityAndExclusion(t *testing.T) {
	s, _ := fixture(t, nil)
	in := input(candidate("a", "10", "1", ""), candidate("b", "5", "1", ""), candidate("c", "0", "1", ""))
	for _, want := range []string{"a", "b", "c"} {
		v := selectOne(t, s, in)
		if v.CandidateId != want {
			t.Fatal(v)
		}
		report(t, s, v, nil)
		in.ExcludedIds = append(in.ExcludedIds, want)
	}
	if _, e := s.Select(background, in); e == nil {
		t.Fatal("selected excluded candidate")
	}
}
func TestWeightDistribution(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		ratio float64
	}{{"2", "1", 2}, {"10", "1", 10}, {"0", "0", 1}, {"10", "0", math.Inf(1)}} {
		t.Run(tc.a+":"+tc.b, func(t *testing.T) {
			s, _ := fixture(t, nil)
			in := input(candidate("a", "0", tc.a, ""), candidate("b", "0", tc.b, ""))
			a := 0
			const n = 20000
			for range n {
				v := selectOne(t, s, in)
				if v.CandidateId == "a" {
					a++
				}
				report(t, s, v, nil)
			}
			if math.IsInf(tc.ratio, 1) {
				if a != n {
					t.Fatal("zero-weight candidate got new traffic")
				}
				return
			}
			want := tc.ratio / (1 + tc.ratio)
			if math.Abs(float64(a)/n-want) > .02 {
				t.Fatalf("got %d/%d expected fraction %f", a, n, want)
			}
		})
	}
}
func TestHealthFeedback(t *testing.T) {
	s, clock := fixture(t, func(c *Config) { c.Health.FailureThreshold = 2; c.Health.Alpha = .5 })
	only := input(candidate("a", "10", "1", ""))
	pool := input(only.Candidates[0], candidate("b", "5", "1", ""))
	report(t, s, selectOne(t, s, only), &sdk.Failure{Kind: sdk.Transport})
	if s.states["a"].errorRate != .5 || s.states["a"].failures != 1 {
		t.Fatal(s.states["a"])
	}
	report(t, s, selectOne(t, s, only), &sdk.Failure{Kind: sdk.Timeout})
	v := selectOne(t, s, pool)
	if v.CandidateId != "b" || v.Reason != "priority_fallback" {
		t.Fatal(v)
	}
	report(t, s, v, nil)
	clock.Add(int64(6 * time.Second))
	v = selectOne(t, s, pool)
	if v.CandidateId != "a" {
		t.Fatal("cooldown did not expire")
	}
	report(t, s, v, nil)
	h := s.states["a"]
	if h.errorRate >= .75 || h.consecutive != 0 || h.firstByte != 10 || h.successes != 1 {
		t.Fatal(h)
	}
}
func TestHealthWeightedPenalty(t *testing.T) {
	s, _ := fixture(t, func(c *Config) { c.Health.Alpha = .8; c.Health.FailureThreshold = 1000 })
	a, b := candidate("a", "0", "1", ""), candidate("b", "0", "1", "")
	report(t, s, selectOne(t, s, input(a)), &sdk.Failure{Kind: sdk.ProtocolError})
	counts := map[string]int{}
	for range 10000 {
		v := selectOne(t, s, input(a, b))
		counts[v.CandidateId]++
		report(t, s, v, &sdk.Failure{Kind: sdk.Downstream})
	}
	if counts["b"] < 4*counts["a"] {
		t.Fatal(counts)
	}
}
func TestNeutralAndFailedSamples(t *testing.T) {
	cases := []struct {
		kind   string
		status int32
		failed bool
	}{{sdk.Downstream, 503, false}, {sdk.Local, 503, false}, {sdk.PluginError, 503, false}, {sdk.HTTP, 400, false}, {sdk.HTTP, 404, false}, {sdk.HTTP, 429, true}, {sdk.HTTP, 500, true}, {sdk.HTTP, 401, true}, {sdk.HTTP, 403, true}, {sdk.Transport, 0, true}, {sdk.Timeout, 0, true}, {sdk.ProtocolError, 0, true}}
	for _, tc := range cases {
		t.Run(tc.kind+string(rune(tc.status)), func(t *testing.T) {
			s, _ := fixture(t, nil)
			v := selectOne(t, s, input(candidate("a", "", "", "")))
			report(t, s, v, &sdk.Failure{Kind: tc.kind, Status: tc.status})
			if (s.states["a"].failures > 0) != tc.failed {
				t.Fatal(s.states["a"])
			}
		})
	}
}
func TestCapacityParallelAndIdempotentRelease(t *testing.T) {
	for _, tc := range []struct {
		cap      string
		limit, n int
	}{{"1", 1, 100}, {"10", 10, 1000}, {"", 1000, 1000}} {
		t.Run("cap"+tc.cap, func(t *testing.T) {
			s, _ := fixture(t, nil)
			in := input(candidate("a", "", "", tc.cap))
			var wg sync.WaitGroup
			var mu sync.Mutex
			var leases []*sdk.Selection
			for range tc.n {
				wg.Go(func() {
					v, e := s.Select(background, in)
					if e == nil {
						mu.Lock()
						leases = append(leases, v)
						mu.Unlock()
					}
				})
			}
			wg.Wait()
			if len(leases) != tc.limit {
				t.Fatalf("admitted %d want %d", len(leases), tc.limit)
			}
			for _, v := range leases {
				for range 2 {
					wg.Go(func() { report(t, s, v, nil) })
				}
			}
			wg.Wait()
			if s.states["a"].reserved != 0 || len(s.leases) != 0 {
				t.Fatal("reservation leaked or negative")
			}
		})
	}
	s, _ := fixture(t, nil)
	c := candidate("a", "", "", "10")
	c.InFlight = 9
	v := selectOne(t, s, input(c))
	if _, e := s.Select(background, input(c)); e == nil {
		t.Fatal("snapshot race oversubscribed")
	}
	report(t, s, v, nil)
}
func TestLeaseOutcomesExpiryAndBounds(t *testing.T) {
	for _, kind := range []string{"", sdk.Transport, sdk.Downstream, sdk.Timeout} {
		s, _ := fixture(t, nil)
		v := selectOne(t, s, input(candidate("a", "", "", "1")))
		var f *sdk.Failure
		if kind != "" {
			f = &sdk.Failure{Kind: kind}
		}
		report(t, s, v, f)
		if s.states["a"].reserved != 0 {
			t.Fatal(kind)
		}
	}
	s, clock := fixture(t, func(c *Config) { c.LeaseTTL = Duration(time.Second); c.MaxLeases = 1 })
	in := input(candidate("a", "", "", "1"))
	v := selectOne(t, s, in)
	if _, e := s.Select(background, in); e == nil {
		t.Fatal("lease cap bypassed")
	}
	clock.Add(int64(2 * time.Second))
	s.mu.Lock()
	s.cleanup(s.now())
	s.mu.Unlock()
	next := selectOne(t, s, in)
	report(t, s, v, nil)
	if s.states["a"].reserved != 1 {
		t.Fatal("expired duplicate released new lease")
	}
	report(t, s, next, nil)
	real, _ := New(func() Config {
		c := Defaults()
		c.LeaseTTL = Duration(5 * time.Millisecond)
		c.CleanupInterval = Duration(time.Millisecond)
		return c
	}())
	defer real.Close()
	selectOne(t, real, in)
	deadline := time.Now().Add(time.Second)
	for {
		real.mu.Lock()
		n := len(real.leases)
		real.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cleanup loop did not expire lease")
		}
		time.Sleep(time.Millisecond)
	}
}

type cancelAfterReserve struct {
	context.Context
	calls int
}

func (c *cancelAfterReserve) Err() error {
	c.calls++
	if c.calls >= 3 {
		return context.Canceled
	}
	return nil
}
func TestCancellation(t *testing.T) {
	s, _ := fixture(t, nil)
	in := input(candidate("a", "", "", "1"))
	ctx, cancel := context.WithCancel(background)
	cancel()
	if _, e := s.Select(ctx, in); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := s.Select(&cancelAfterReserve{Context: background}, in); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if len(s.leases) != 0 || s.states["a"].reserved != 0 {
		t.Fatal("cancelled Select leaked reservation")
	}
	v := selectOne(t, s, in)
	e := s.Report(ctx, &sdk.AttemptResult{RouteId: "route", CandidateId: "a", Lease: v.Lease, Status: 200})
	if !errors.Is(e, context.Canceled) || s.states["a"].reserved != 0 || s.states["a"].successes != 0 {
		t.Fatal("cancelled report must release without learning")
	}
}
func TestStickyLifecycleAndBounds(t *testing.T) {
	s, clock := fixture(t, func(c *Config) { c.Sticky.TTL = Duration(time.Second); c.Sticky.MaxEntries = 2 })
	a, b := candidate("a", "", "", ""), candidate("b", "", "", "")
	in := input(a)
	affinity(in, "session")
	v := selectOne(t, s, in)
	if len(s.sticky) != 0 {
		t.Fatal("bound before success")
	}
	report(t, s, v, &sdk.Failure{Kind: sdk.Downstream})
	if len(s.sticky) != 0 {
		t.Fatal("bound failed request")
	}
	report(t, s, selectOne(t, s, in), nil)
	pool := input(a, b)
	affinity(pool, "session")
	v = selectOne(t, s, pool)
	if v.CandidateId != "a" || !v.AffinityHit || v.Reason != "sticky_hit" {
		t.Fatal(v)
	}
	report(t, s, v, nil)
	pool.ExcludedIds = []string{"a"}
	v = selectOne(t, s, pool)
	if v.CandidateId != "b" || v.AffinityHit {
		t.Fatal(v)
	}
	report(t, s, v, nil)
	clock.Add(int64(2 * time.Second))
	pool.ExcludedIds = nil
	v = selectOne(t, s, pool)
	if v.AffinityHit {
		t.Fatal("TTL bypassed")
	}
	report(t, s, v, nil)
	for _, key := range []string{"old", "new", "newest"} {
		x := input(a)
		affinity(x, key)
		report(t, s, selectOne(t, s, x), nil)
	}
	x := input(a)
	affinity(x, "old")
	key, _ := s.affinity(x)
	if len(s.sticky) != 2 || s.sticky[key] != nil {
		t.Fatal("LRU bound violated")
	}
	x.Request.Metadata[sdk.AffinityKey] = string(make([]byte, 513))
	v = selectOne(t, s, x)
	if s.leases[string(v.Lease)].hasAffinity {
		t.Fatal("oversized affinity accepted")
	}
	report(t, s, v, nil)
}
func TestStickyEscapeAndStrict(t *testing.T) {
	for _, mode := range []string{"prefer", "strict"} {
		for _, why := range []string{"capacity", "unhealthy", "excluded"} {
			t.Run(mode+why, func(t *testing.T) {
				s, _ := fixture(t, func(c *Config) { c.Sticky.Mode = mode; c.Health.FailureThreshold = 1 })
				a, b := candidate("a", "", "", "1"), candidate("b", "", "", "10")
				in := input(a)
				affinity(in, "session")
				report(t, s, selectOne(t, s, in), nil)
				pool := input(a, b)
				affinity(pool, "session")
				switch why {
				case "capacity":
					a.InFlight = 1
				case "excluded":
					pool.ExcludedIds = []string{"a"}
				case "unhealthy":
					v := selectOne(t, s, in)
					report(t, s, v, &sdk.Failure{Kind: sdk.Transport})
				}
				v, e := s.Select(background, pool)
				if mode == "strict" {
					if e == nil {
						t.Fatal("strict binding escaped")
					}
					return
				}
				if e != nil || v.CandidateId != "b" || v.Reason != "sticky_escape_"+why {
					t.Fatalf("%v %v", v, e)
				}
				report(t, s, v, nil)
			})
		}
	}
}
func TestConfigAndMalformedHints(t *testing.T) {
	for _, raw := range []string{`{}`, `{"sticky":{"mode":"off"},"health":{"enabled":false}}`} {
		if _, e := ParseConfig([]byte(raw)); e != nil {
			t.Fatal(e)
		}
	}
	for _, raw := range []string{`null`, `{`, `{"health":{"ewma_alpha":0}}`, `{"cleanup_interval":"1h"}`, `{"unknown":1}`, `{"sticky":{"mode":"forever"}}`} {
		if _, e := ParseConfig([]byte(raw)); e == nil {
			t.Fatal(raw)
		}
	}
	s, _ := fixture(t, nil)
	for key, value := range map[string]string{sdk.SchedulerPriority: "NaN", sdk.SchedulerWeight: "-1", sdk.SchedulerCapacity: "-2"} {
		c := candidate("a", "", "", "")
		c.Metadata[key] = value
		if _, e := s.Select(background, input(c)); e == nil {
			t.Fatal(key)
		}
	}
}

func TestStickyPriorityWeightAndMode(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		mutate func(*sdk.Candidate)
		want   string
	}{
		{"off", func(c *sdk.Candidate) {}, "a"},
		{"prefer", func(c *sdk.Candidate) { c.Metadata[sdk.SchedulerPriority] = "-1" }, "b"},
		{"prefer", func(c *sdk.Candidate) { c.Metadata[sdk.SchedulerWeight] = "0" }, "b"},
	} {
		s, _ := fixture(t, func(c *Config) { c.Sticky.Mode = tc.mode })
		a := candidate("a", "0", "1", "")
		in := input(a)
		affinity(in, "key")
		report(t, s, selectOne(t, s, in), nil)
		tc.mutate(a)
		in.Candidates = append(in.Candidates, candidate("b", "0", "1", ""))
		v := selectOne(t, s, in)
		if tc.mode == "off" {
			if v.AffinityHit || len(s.sticky) != 0 {
				t.Fatal("off mode stored affinity")
			}
		} else if v.CandidateId != tc.want || v.AffinityHit {
			t.Fatal(v)
		}
		report(t, s, v, nil)
	}
}
func TestBoundedStateAndStaleHealth(t *testing.T) {
	s, clock := fixture(t, func(c *Config) { c.MaxCandidates = 1; c.StateTTL = Duration(6 * time.Minute) })
	a := input(candidate("a", "", "", ""))
	v := selectOne(t, s, a)
	report(t, s, v, &sdk.Failure{Kind: sdk.Transport})
	if _, e := s.Select(background, input(candidate("b", "", "", ""))); e == nil {
		t.Fatal("unbounded candidate health")
	}
	clock.Add(int64(7 * time.Minute))
	v = selectOne(t, s, input(candidate("b", "", "", "")))
	if len(s.states) != 1 || s.states["a"] != nil {
		t.Fatal("stale health did not expire")
	}
	report(t, s, v, nil)
}
