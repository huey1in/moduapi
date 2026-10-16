// Package scheduler implements generic policy and process-local admission.
// It has no provider, credential, account or kernel dependency.
package scheduler

import (
	"container/list"
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/huey1in/moduapi/sdk"
)

type healthState struct {
	successes, failures                  uint64
	errorRate, firstByte                 float64
	consecutive                          int
	lastFailure, cooldownUntil, lastSeen time.Time
	reserved                             int64
	durationMS, bytesEmitted             int64
}
type lease struct {
	candidate, route string
	created, expires time.Time
	affinity         [32]byte
	hasAffinity      bool
}
type stickyEntry struct {
	key       [32]byte
	candidate string
	expires   time.Time
}
type choice struct {
	id                                   string
	priority, weight, capacity, inFlight int64
	state                                *healthState
	factor                               float64
	blocked                              string
}
type Scheduler struct {
	mu         sync.Mutex
	cfg        Config
	states     map[string]*healthState
	leases     map[string]lease
	sticky     map[[32]byte]*list.Element
	lru        *list.List
	secret     [32]byte
	rng        *rand.Rand
	now        func() time.Time
	stop, done chan struct{}
	closed     bool
}

func New(c Config) (*Scheduler, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	s := &Scheduler{cfg: c, states: map[string]*healthState{}, leases: map[string]lease{}, sticky: map[[32]byte]*list.Element{}, lru: list.New(), rng: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())), now: time.Now, stop: make(chan struct{}), done: make(chan struct{})}
	if _, e := cryptorand.Read(s.secret[:]); e != nil {
		return nil, e
	}
	go s.cleaner()
	return s, nil
}
func (s *Scheduler) Close() error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.stop)
	}
	s.mu.Unlock()
	<-s.done
	return nil
}
func (s *Scheduler) cleaner() {
	defer close(s.done)
	ticker := time.NewTicker(time.Duration(s.cfg.CleanupInterval))
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			s.cleanup(s.now())
			s.mu.Unlock()
		}
	}
}
func (s *Scheduler) release(id string, l lease) {
	delete(s.leases, id)
	if h := s.states[l.candidate]; h != nil && h.reserved > 0 {
		h.reserved--
	}
}
func (s *Scheduler) cleanup(now time.Time) {
	for id, l := range s.leases {
		if !now.Before(l.expires) {
			s.release(id, l)
		}
	}
	for _, e := range s.sticky {
		if !now.Before(e.Value.(*stickyEntry).expires) {
			s.evict(e)
		}
	}
	for id, h := range s.states {
		if h.reserved == 0 && now.Sub(h.lastSeen) >= time.Duration(s.cfg.StateTTL) {
			delete(s.states, id)
		}
	}
}
func (s *Scheduler) evict(e *list.Element) {
	delete(s.sticky, e.Value.(*stickyEntry).key)
	s.lru.Remove(e)
}
func (s *Scheduler) affinity(in *sdk.SelectionInput) ([32]byte, bool) {
	var zero [32]byte
	key := in.GetRequest().GetMetadata()[sdk.AffinityKey]
	if s.cfg.Sticky.Mode == "off" || len(key) == 0 || len(key) > 512 {
		return zero, false
	}
	h := hmac.New(sha256.New, s.secret[:])
	h.Write([]byte(in.RouteId))
	h.Write([]byte{0})
	h.Write([]byte(key))
	copy(zero[:], h.Sum(nil))
	return zero, true
}
func hint(m map[string]string, key string, def int64) (int64, error) {
	v, ok := m[key]
	if !ok {
		return def, nil
	}
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil {
		return 0, fmt.Errorf("invalid scheduler hint")
	}
	return n, nil
}
func (s *Scheduler) Select(ctx context.Context, in *sdk.SelectionInput) (*sdk.Selection, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if in == nil || len(in.Candidates) == 0 || len(in.Candidates) > s.cfg.MaxCandidates || len(in.RouteId) > 256 {
		return nil, sdk.Fail(sdk.Local, "invalid_candidates", "invalid candidate set", 400)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if s.closed {
		return nil, sdk.Fail(sdk.PluginError, "scheduler_closed", "scheduler closed", 503)
	}
	now := s.now()
	// At table pressure, reclaim stale entries synchronously as well as on ticks.
	if len(s.leases) >= s.cfg.MaxLeases || len(s.states)+len(in.Candidates) > s.cfg.MaxCandidates {
		s.cleanup(now)
	}
	if len(s.leases) >= s.cfg.MaxLeases {
		return nil, sdk.Fail(sdk.Local, "lease_limit", "scheduler lease limit reached", 503)
	}
	excluded := make(map[string]bool, len(in.ExcludedIds))
	for _, id := range in.ExcludedIds {
		excluded[id] = true
	}
	choices := make([]choice, 0, len(in.Candidates))
	positive := map[int64]bool{}
	seen := map[string]bool{}
	top := int64(math.MinInt64)
	for _, c := range in.Candidates {
		if c == nil || c.Id == "" || len(c.Id) > 256 || seen[c.Id] || c.InFlight < 0 {
			return nil, sdk.Fail(sdk.Local, "invalid_candidates", "invalid candidate set", 400)
		}
		seen[c.Id] = true
		p, e1 := hint(c.Metadata, sdk.SchedulerPriority, 0)
		w, e2 := hint(c.Metadata, sdk.SchedulerWeight, 1)
		cap, e3 := hint(c.Metadata, sdk.SchedulerCapacity, 0)
		if e1 != nil || e2 != nil || e3 != nil || w < 0 || w > 1000000000 || cap < 0 || cap > 1000000000 {
			return nil, sdk.Fail(sdk.Local, "invalid_scheduler_hint", "invalid scheduler hint", 400)
		}
		h := s.states[c.Id]
		if h == nil {
			if len(s.states) >= s.cfg.MaxCandidates {
				return nil, sdk.Fail(sdk.Local, "state_limit", "scheduler state limit reached", 503)
			}
			h = &healthState{}
			s.states[c.Id] = h
		}
		h.lastSeen = now
		q := choice{id: c.Id, priority: p, weight: w, capacity: cap, inFlight: c.InFlight, state: h, factor: 1}
		if s.cfg.Health.Enabled {
			q.factor = math.Max(s.cfg.Health.MinWeightFactor, 1-h.errorRate)
		}
		switch {
		case excluded[c.Id]:
			q.blocked = "excluded"
		case s.cfg.Health.Enabled && now.Before(h.cooldownUntil):
			q.blocked = "unhealthy"
		case cap > 0 && (c.InFlight >= cap || h.reserved >= cap-c.InFlight):
			q.blocked = "capacity"
		}
		if !excluded[c.Id] {
			top = max(top, p)
			positive[p] = positive[p] || w > 0
		}
		choices = append(choices, q)
	}
	pool := make([]*choice, 0, len(choices))
	tier := int64(math.MinInt64)
	for i := range choices {
		q := &choices[i]
		if q.blocked == "" && q.weight == 0 && positive[q.priority] {
			q.blocked = "weight"
		}
		if q.blocked != "" {
			continue
		}
		if q.priority > tier {
			tier = q.priority
			pool = pool[:0]
		}
		if q.priority == tier {
			pool = append(pool, q)
		}
	}
	key, hasKey := s.affinity(in)
	reason := ""
	var selected *choice
	hit := false
	if hasKey {
		if entry := s.sticky[key]; entry != nil {
			v := entry.Value.(*stickyEntry)
			if !now.Before(v.expires) {
				s.evict(entry)
			} else {
				s.lru.MoveToFront(entry)
				why := "missing"
				for i := range choices {
					q := &choices[i]
					if q.id != v.candidate {
						continue
					}
					why = q.blocked
					if why == "" && q.priority != tier {
						why = "priority"
					}
					if why == "" && s.cfg.Health.Enabled && q.state.consecutive > 0 {
						why = "unhealthy"
					}
					if why == "" {
						selected = q
						hit = true
						reason = "sticky_hit"
					}
					break
				}
				if !hit {
					if s.cfg.Sticky.Mode == "strict" || (why == "unhealthy" && !s.cfg.Sticky.EscapeUnhealthy) || (why == "capacity" && !s.cfg.Sticky.EscapeCapacity) {
						return nil, sdk.Fail(sdk.Local, "sticky_unavailable", "bound candidate is unavailable", 503)
					}
					reason = "sticky_escape_" + why
				}
			}
		}
	}
	if selected == nil {
		if len(pool) == 0 {
			return nil, sdk.Fail(sdk.Local, "no_candidate", "no eligible candidate", 503)
		}
		if reason == "" {
			reason = "priority_weight"
			if tier < top {
				reason = "priority_fallback"
			} else {
				for _, q := range choices {
					if q.priority == tier && q.blocked == "capacity" {
						reason = "capacity_fallback"
						break
					}
					if q.priority == tier && q.factor < 1 {
						reason = "health_penalty"
					}
				}
			}
		}
		selected = weighted(pool, !positive[tier], s.rng)
	}
	var id [16]byte
	if _, e := cryptorand.Read(id[:]); e != nil {
		return nil, e
	}
	l := lease{selected.id, in.RouteId, now, now.Add(time.Duration(s.cfg.LeaseTTL)), key, hasKey}
	s.leases[string(id[:])] = l
	selected.state.reserved++
	// Cancellation after reservation but before return does not depend on Report.
	if e := ctx.Err(); e != nil {
		s.release(string(id[:]), l)
		return nil, e
	}
	return &sdk.Selection{CandidateId: selected.id, Reason: reason, AffinityHit: hit, Lease: id[:]}, nil
}

func sample(r *sdk.AttemptResult) int {
	if f := r.Failure; f != nil {
		switch f.Kind {
		case sdk.Transport, sdk.Timeout, sdk.ProtocolError:
			return -1
		case sdk.HTTP:
		default:
			return 0
		}
	}
	status := r.Status
	if r.Failure != nil && r.Failure.Status != 0 {
		status = r.Failure.Status
	}
	if status == 401 || status == 403 || status == 429 || status >= 500 {
		return -1
	}
	if r.Failure == nil && status >= 200 && status < 400 {
		return 1
	}
	return 0
}
func (s *Scheduler) Report(ctx context.Context, r *sdk.AttemptResult) error {
	if r == nil {
		return sdk.Fail(sdk.Local, "invalid_report", "invalid scheduler report", 400)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.leases[string(r.Lease)]
	if !ok {
		return ctx.Err()
	}
	if l.candidate != r.CandidateId || l.route != r.RouteId {
		return sdk.Fail(sdk.Local, "invalid_lease", "lease does not match attempt", 400)
	}
	s.release(string(r.Lease), l)
	if e := ctx.Err(); e != nil {
		return e
	}
	now := s.now()
	if !now.Before(l.expires) {
		return nil
	}
	h := s.states[r.CandidateId]
	h.lastSeen = now
	h.durationMS = max(0, r.DurationMs)
	h.bytesEmitted = max(0, r.BytesEmitted)
	outcome := sample(r)
	if outcome != 0 && s.cfg.Health.Enabled {
		alpha := s.cfg.Health.Alpha
		value := 0.0
		if outcome < 0 {
			value = 1
			h.failures++
			h.consecutive++
			h.lastFailure = now
			if h.consecutive >= s.cfg.Health.FailureThreshold {
				h.cooldownUntil = now.Add(time.Duration(s.cfg.Health.Cooldown))
			}
		} else {
			h.successes++
			h.consecutive = 0
			h.cooldownUntil = time.Time{}
		}
		h.errorRate = alpha*value + (1-alpha)*h.errorRate
		if r.FirstByteMs > 0 {
			if h.firstByte == 0 {
				h.firstByte = float64(r.FirstByteMs)
			} else {
				h.firstByte = alpha*float64(r.FirstByteMs) + (1-alpha)*h.firstByte
			}
		}
	}
	if outcome > 0 && l.hasAffinity {
		s.bind(l.affinity, r.CandidateId, now)
	}
	return nil
}
func (s *Scheduler) bind(key [32]byte, id string, now time.Time) {
	if entry := s.sticky[key]; entry != nil {
		v := entry.Value.(*stickyEntry)
		v.candidate = id
		v.expires = now.Add(time.Duration(s.cfg.Sticky.TTL))
		s.lru.MoveToFront(entry)
		return
	}
	for len(s.sticky) >= s.cfg.Sticky.MaxEntries {
		s.evict(s.lru.Back())
	}
	s.sticky[key] = s.lru.PushFront(&stickyEntry{key, id, now.Add(time.Duration(s.cfg.Sticky.TTL))})
}

func weighted(pool []*choice, allZero bool, rng *rand.Rand) *choice {
	total := 0.0
	for _, q := range pool {
		w := q.weight
		if allZero {
			w = 1
		}
		total += float64(w) * q.factor
	}
	roll := rng.Float64() * total
	for _, q := range pool {
		w := q.weight
		if allZero {
			w = 1
		}
		roll -= float64(w) * q.factor
		if roll < 0 {
			return q
		}
	}
	return pool[len(pool)-1]
}
