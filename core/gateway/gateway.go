// Package gateway coordinates the kernel lifecycle using SDK contracts only.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/core/lifecycle"
	"github.com/huey1in/moduapi/core/request"
	"github.com/huey1in/moduapi/core/retry"
	"github.com/huey1in/moduapi/core/router"
	"github.com/huey1in/moduapi/core/streaming"
	"github.com/huey1in/moduapi/internal/httpwire"
	"github.com/huey1in/moduapi/sdk"
)

type Gateway struct {
	cfg      config.Config
	registry sdk.Registry
	router   *router.Router
	hooks    *lifecycle.Bus
	policy   retry.Policy
	load     map[string]*atomic.Int64
}

func New(c config.Config, reg sdk.Registry, hooks *lifecycle.Bus, policy retry.Policy) (*Gateway, error) {
	c.Defaults()
	if e := c.Validate(); e != nil {
		return nil, e
	}
	r, e := router.New(c.Routes)
	if e != nil {
		return nil, e
	}
	if policy == nil {
		policy = retry.Conservative{}
	}
	g := &Gateway{cfg: c, registry: reg, router: r, hooks: hooks, policy: policy, load: map[string]*atomic.Int64{}}
	for id := range c.Upstreams {
		g.load[id] = &atomic.Int64{}
	}
	return g, nil
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"status\":\"ok\"}\n")
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/readyz" {
		w.Header().Set("Content-Type", "application/json")
		if !g.registry.Ready() {
			w.WriteHeader(503)
			_, _ = io.WriteString(w, "{\"status\":\"unavailable\"}\n")
		} else {
			_, _ = io.WriteString(w, "{\"status\":\"ready\"}\n")
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(g.cfg.Server.RequestTimeout))
	defer cancel()
	state := request.New(ctx)
	w.Header().Set("X-Request-ID", state.ID)
	controller := http.NewResponseController(w)
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = controller.SetWriteDeadline(time.Now()); close(done) })
	var once sync.Once
	finishWatch := func() {
		once.Do(func() {
			if !stop() {
				<-done
			}
		})
	}
	defer finishWatch()
	usage, err := g.handle(w, r, state)
	var failure *sdk.Failure
	if err != nil {
		failure = g.failure(ctx, err)
	} else if len(state.Attempts) > 0 {
		failure = state.Attempts[len(state.Attempts)-1].Failure
	}
	info := &sdk.RequestInfo{Id: state.ID}
	if state.Request != nil {
		info = sdk.Info(state.Request)
	}
	u := &sdk.UsageEvent{RequestID: state.ID, Model: info.Model, Duration: time.Since(state.StartedAt), Partial: failure != nil}
	if usage != nil {
		u.InputTokens = usage.InputTokens
		u.OutputTokens = usage.OutputTokens
		u.CachedTokens = usage.CachedTokens
		u.Partial = u.Partial || usage.Partial
	}
	post, postCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	if e := g.hooks.Emit(post, lifecycle.Event{Stage: lifecycle.Complete, Request: info, Usage: u, Failure: failure}); e != nil {
		slog.Warn("completion hook failed", "request_id", state.ID)
	}
	postCancel()
	finishWatch()
	if err != nil {
		if state.Committed {
			panic(http.ErrAbortHandler)
		}
		if r.Context().Err() != nil {
			return
		}
		_ = controller.SetWriteDeadline(time.Now().Add(time.Duration(g.cfg.Server.WriteTimeout)))
		writeFailure(w, state.ID, failure)
	}
}
func (g *Gateway) handle(w http.ResponseWriter, r *http.Request, s *request.Context) (*sdk.Usage, error) {
	ctx := s.Context
	pluginID, protocolID, ok := g.registry.Endpoint(r.Method, r.URL.Path)
	if !ok {
		return nil, sdk.Fail(sdk.Local, "endpoint_not_found", "unknown endpoint", 404)
	}
	if e := g.hooks.Emit(ctx, lifecycle.Event{Stage: lifecycle.Ingress, Request: &sdk.RequestInfo{Id: s.ID}}); e != nil {
		return nil, e
	}
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(time.Duration(g.cfg.Server.RequestTimeout)))
	body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, g.cfg.Server.MaxBodyBytes))
	if e != nil {
		var large *http.MaxBytesError
		if errors.As(e, &large) {
			return nil, sdk.Fail(sdk.Local, "body_too_large", "request body exceeds configured limit", 413)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, sdk.Fail(sdk.Local, "body_read_failed", "could not read request body", 400)
	}
	_ = http.NewResponseController(w).SetReadDeadline(time.Time{})
	protocol, e := g.registry.Protocol(pluginID)
	if e != nil {
		return nil, e
	}
	canonical, e := protocol.Decode(ctx, &sdk.Ingress{Id: s.ID, Method: r.Method, Path: r.URL.Path, Headers: sdk.Headers(r.Header), Body: body})
	if e != nil {
		return nil, e
	}
	if canonical == nil || canonical.Id != s.ID || canonical.Protocol != protocolID || canonical.Model == "" || len(canonical.Body) > 8<<20 {
		return nil, sdk.Fail(sdk.PluginError, "invalid_canonical", "protocol plugin returned an invalid request", 502)
	}
	s.Request = canonical
	if e = g.hooks.Emit(ctx, lifecycle.Event{Stage: lifecycle.Decoded, Request: sdk.Info(canonical)}); e != nil {
		return nil, e
	}
	route, e := g.router.Match(ctx, canonical)
	if e != nil {
		return nil, e
	}
	if e = g.hooks.Emit(ctx, lifecycle.Event{Stage: lifecycle.Routed, Request: sdk.Info(canonical)}); e != nil {
		return nil, e
	}
	scheduler, e := g.registry.Scheduler(route.Scheduler)
	if e != nil {
		return nil, e
	}
	var excluded []string
	maxAttempts := min(route.Retry.MaxAttempts, len(route.Upstreams))
	for number := 1; number <= maxAttempts; number++ {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		var candidates []*sdk.Candidate
		for _, id := range route.Upstreams {
			candidates = append(candidates, &sdk.Candidate{Id: id, Metadata: maps.Clone(g.cfg.Upstreams[id].Metadata), InFlight: g.load[id].Load()})
		}
		selected, e := scheduler.Select(ctx, &sdk.SelectionInput{RouteId: route.ID, Request: sdk.Info(canonical), Candidates: candidates, ExcludedIds: slices.Clone(excluded), Attempt: uint32(number)})
		if e != nil {
			return nil, e
		}
		if selected == nil || !slices.Contains(route.Upstreams, selected.CandidateId) || slices.Contains(excluded, selected.CandidateId) {
			return nil, sdk.Fail(sdk.PluginError, "invalid_selection", "scheduler selected an ineligible candidate", 502)
		}
		result := &sdk.AttemptResult{RequestId: s.ID, RouteId: route.ID, CandidateId: selected.CandidateId, Attempt: uint32(number), Lease: selected.Lease}
		usage, e := g.attempt(w, s, route, maxAttempts, protocol, scheduler, result)
		s.Attempts = append(s.Attempts, result)
		if e == nil {
			return usage, nil
		}
		decision := g.retryDecision(ctx, retry.State{Attempt: number, MaxAttempts: maxAttempts, AllowReplay: route.Retry.AllowReplay, Committed: s.Committed, Failure: result.Failure})
		if !decision.Retry {
			return usage, &sdk.Error{Failure: result.Failure}
		}
		excluded = append(excluded, selected.CandidateId)
		if decision.Delay > 0 {
			timer := time.NewTimer(decision.Delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return usage, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, sdk.Fail(sdk.Local, "retry_exhausted", "retry budget exhausted", 503)
}
func (g *Gateway) attempt(w http.ResponseWriter, s *request.Context, route router.Route, budget int, protocol sdk.ProtocolClient, scheduler sdk.Scheduler, result *sdk.AttemptResult) (usage *sdk.Usage, err error) {
	started := time.Now()
	before := s.BytesEmitted
	g.load[result.CandidateId].Add(1)
	ctx, cancel := context.WithCancelCause(s.Context)
	defer cancel(nil)
	defer func() {
		g.load[result.CandidateId].Add(-1)
		result.DurationMs = time.Since(started).Milliseconds()
		result.Committed = s.Committed
		result.BytesEmitted = s.BytesEmitted - before
		if err != nil {
			result.Failure = g.failure(ctx, err)
			if result.Failure.Usage != nil {
				usage = result.Failure.Usage
			}
		} else if result.Status >= 400 {
			result.Failure = httpFailure(result.Status)
		}
		cleanup, finish := context.WithTimeout(context.WithoutCancel(s.Context), time.Second)
		defer finish()
		if e := scheduler.Report(cleanup, result); e != nil {
			slog.Warn("scheduler feedback failed", "request_id", s.ID)
		}
		if e := g.hooks.Emit(cleanup, lifecycle.Event{Stage: lifecycle.AttemptEnd, Request: sdk.Info(s.Request), Attempt: result, Failure: result.Failure}); e != nil {
			slog.Warn("attempt hook failed", "request_id", s.ID)
		}
	}()
	if e := g.hooks.Emit(ctx, lifecycle.Event{Stage: lifecycle.Selected, Request: sdk.Info(s.Request), Attempt: result}); e != nil {
		return nil, e
	}
	up := g.cfg.Upstreams[result.CandidateId]
	prepared, e := protocol.Prepare(ctx, &sdk.Preparation{Request: s.Request, OptionsJson: up.ProtocolConfig.JSON()})
	if e != nil {
		return nil, e
	}
	if prepared == nil || prepared.Id != s.ID || prepared.Protocol != s.Request.Protocol || prepared.Stream != s.Request.Stream || len(prepared.Body) > 8<<20 {
		return nil, sdk.Fail(sdk.PluginError, "invalid_preparation", "protocol plugin returned invalid preparation", 502)
	}
	transport, e := g.registry.Upstream(up.Plugin)
	if e != nil {
		return nil, e
	}
	timer := time.AfterFunc(time.Duration(g.cfg.Server.FirstByteTimeout), func() { cancel(sdk.Fail(sdk.Timeout, "first_byte_timeout", "upstream first-byte timeout", 504)) })
	defer timer.Stop()
	raw, e := transport.Execute(ctx, &sdk.Execution{Request: prepared, UpstreamId: result.CandidateId, ConfigJson: up.Config.JSON()})
	if e != nil {
		return nil, e
	}
	raw = streaming.Guard(raw)
	defer raw.Close()
	first, e := raw.Recv()
	if e != nil {
		return nil, e
	}
	if f := first.GetFailure(); f != nil {
		return f.Usage, &sdk.Error{Failure: f}
	}
	if first.GetStart() == nil {
		return nil, sdk.Fail(sdk.ProtocolError, "missing_headers", "upstream response has no headers", 502)
	}
	result.Status = first.GetStart().Status
	if result.Status >= 400 {
		f := httpFailure(result.Status)
		if g.retryDecision(ctx, retry.State{Attempt: int(result.Attempt), MaxAttempts: budget, AllowReplay: route.Retry.AllowReplay, Failure: f}).Retry {
			return nil, &sdk.Error{Failure: f}
		}
	}
	transformed, e := protocol.Transform(ctx, prepared, streaming.Prepend(first, raw))
	if e != nil {
		return nil, e
	}
	transformed = streaming.Guard(transformed)
	defer transformed.Close()
	header, e := transformed.Recv()
	if e != nil {
		return nil, e
	}
	if f := header.GetFailure(); f != nil {
		return f.Usage, &sdk.Error{Failure: f}
	}
	if header.GetStart() == nil {
		return nil, sdk.Fail(sdk.ProtocolError, "missing_headers", "protocol response has no headers", 502)
	}
	result.Status = header.GetStart().Status
	// Delay committing headers until first data/end, so first-byte failures remain recoverable.
	for {
		var f *sdk.Frame
		if s.Committed {
			idle := time.AfterFunc(time.Duration(g.cfg.Server.IdleTimeout), func() { cancel(sdk.Fail(sdk.Timeout, "idle_timeout", "upstream idle timeout", 504)) })
			f, e = transformed.Recv()
			idle.Stop()
		} else {
			f, e = transformed.Recv()
		}
		if e != nil {
			return nil, e
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if failure := f.GetFailure(); failure != nil {
			return failure.Usage, &sdk.Error{Failure: failure}
		}
		if f.GetData() != nil && len(f.GetData()) == 0 {
			continue
		}
		if !s.Committed {
			timer.Stop()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			result.FirstByteMs = time.Since(started).Milliseconds()
			for k, v := range httpwire.Clean(sdk.HTTPHeaders(header.GetStart().Headers)) {
				if k != "X-Request-Id" {
					w.Header()[k] = v
				}
			}
			w.Header().Set("X-Request-ID", s.ID)
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Duration(g.cfg.Server.WriteTimeout)))
			w.WriteHeader(int(header.GetStart().Status))
			s.Committed = true
		}
		if end := f.GetEnd(); end != nil {
			return end.Usage, nil
		}
		if b := f.GetData(); b != nil {
			controller := http.NewResponseController(w)
			_ = controller.SetWriteDeadline(time.Now().Add(time.Duration(g.cfg.Server.WriteTimeout)))
			n, we := w.Write(b)
			s.BytesEmitted += int64(n)
			if we != nil {
				return nil, sdk.Fail(sdk.Downstream, "downstream_write", "downstream write failed", 499)
			}
			if s.Request.Stream {
				if e := controller.Flush(); e != nil {
					return nil, sdk.Fail(sdk.Downstream, "downstream_flush", "downstream flush failed", 499)
				}
			}
		}
	}
}

// Policy can narrow replay, but cannot relax kernel safety invariants.
func (g *Gateway) retryDecision(ctx context.Context, s retry.State) retry.Decision {
	if ctx.Err() != nil || s.Committed || !s.AllowReplay || s.Attempt >= s.MaxAttempts {
		return retry.Decision{Reason: "kernel replay boundary"}
	}
	return g.policy.Decide(ctx, s)
}
func (g *Gateway) failure(ctx context.Context, err error) *sdk.Failure {
	if cause := context.Cause(ctx); cause != nil {
		return sdk.FailureOf(cause)
	}
	return sdk.FailureOf(err)
}
func httpFailure(status int32) *sdk.Failure {
	return &sdk.Failure{Kind: sdk.HTTP, Code: "upstream_http_error", Message: "upstream returned an error status", Status: status, RequestSent: true}
}
func writeFailure(w http.ResponseWriter, id string, f *sdk.Failure) {
	status := int(f.Status)
	if status < 400 || status > 599 {
		status = 502
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"type": f.Kind, "code": f.Code, "message": f.Message, "request_id": id}})
}
