package process

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/huey1in/moduapi/runtime/manifest"
)

// Options belong to the process runtime, not candidate scheduling policy.
// Zero-value disables supervision; NewManager uses bounded defaults.
type RestartOptions struct {
	Enabled              bool
	Initial, Max, Window time.Duration
	Jitter               float64
	CrashLimit           int
}

func DefaultRestartOptions() RestartOptions {
	return RestartOptions{true, 250 * time.Millisecond, 30 * time.Second, 60 * time.Second, .2, 5}
}

type restartState struct {
	crashes  []time.Time
	failed   bool
	restarts int
}

func NewManagerWithOptions(o RestartOptions) *Manager {
	// Clamp invalid embedder options to safe defaults rather than allow tight loops.
	defaults := DefaultRestartOptions()
	if o.Initial <= 0 {
		o.Initial = defaults.Initial
	}
	if o.Max < o.Initial {
		o.Max = max(defaults.Max, o.Initial)
	}
	if o.Window <= 0 {
		o.Window = defaults.Window
	}
	if o.CrashLimit < 1 {
		o.CrashLimit = defaults.CrashLimit
	}
	if o.Jitter < 0 || o.Jitter > 1 {
		o.Jitter = defaults.Jitter
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{items: map[string]*Process{}, manifests: map[string]manifest.Manifest{}, configs: map[string]json.RawMessage{}, restart: o, states: map[string]*restartState{}, ctx: ctx, cancel: cancel, watchers: sync.WaitGroup{}}
}
func (o RestartOptions) delay(crashes int) time.Duration {
	d := o.Initial
	for i := 1; i < crashes && d < o.Max; i++ {
		if d > o.Max/2 {
			d = o.Max
			break
		}
		d *= 2
	}
	d = min(d, o.Max)
	return min(o.Max, time.Duration(float64(d)*(1-o.Jitter+rand.Float64()*2*o.Jitter)))
}
func (m *Manager) watch(p *Process) {
	if !m.restart.Enabled {
		return
	}
	m.watchers.Go(func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-p.Done():
			}
			if p.stopping.Load() {
				return
			}
			id := p.Manifest.ID
			m.mu.Lock()
			if m.closed || m.items[id] != p {
				m.mu.Unlock()
				return
			}
			state := m.states[id]
			if state == nil {
				state = &restartState{}
				m.states[id] = state
			}
			now := time.Now()
			kept := state.crashes[:0]
			for _, at := range state.crashes {
				if now.Sub(at) < m.restart.Window {
					kept = append(kept, at)
				}
			}
			state.crashes = append(kept, now)
			n := len(state.crashes)
			state.failed = n >= m.restart.CrashLimit
			m.mu.Unlock()
			// Dead process handles/connection are closed even when protection stops restart.
			p.Stop(context.Background())
			if n >= m.restart.CrashLimit {
				return
			}
			timer := time.NewTimer(m.restart.delay(n))
			select {
			case <-m.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			m.ops.Lock()
			m.mu.RLock()
			current, closed := m.items[id], m.closed
			m.mu.RUnlock()
			if closed || current != p || m.ctx.Err() != nil {
				m.ops.Unlock()
				return
			}
			err := m.replace(m.ctx, id, p)
			m.mu.Lock()
			if err != nil {
				state.failed = true
			} else {
				state.restarts++
				p = m.items[id]
			}
			m.mu.Unlock()
			m.ops.Unlock()
			// A replacement that never became ready is a startup failure, not a new crash.
			if err != nil {
				return
			}
		}
	})
}

type Diagnostics struct {
	ID            string
	PID           int
	Ready, Failed bool
	Uptime        time.Duration
	Restarts      int
	StderrTail    string
}

func (m *Manager) Diagnostics(id string) (Diagnostics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.items[id]
	if !ok {
		return Diagnostics{}, false
	}
	d := Diagnostics{ID: id, PID: p.PID(), Ready: p.Ready(), Uptime: time.Since(p.StartedAt), StderrTail: p.StderrTail()}
	if s := m.states[id]; s != nil {
		d.Failed = s.failed
		d.Restarts = s.restarts
	}
	return d, true
}
