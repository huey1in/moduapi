package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/runtime/manifest"
	"github.com/huey1in/moduapi/runtime/process"
	"github.com/huey1in/moduapi/runtime/registry"
	"github.com/huey1in/moduapi/sdk"
)

func TestAdvancedSchedulerProcess(t *testing.T) {
	const id = "scheduler.priority-weight-health"
	m, e := manifest.Read(filepath.Join(bundles, id, "manifest.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	manager := process.NewManager()
	defer manager.Close(context.Background())
	if e = manager.Start(context.Background(), []manifest.Manifest{m}, map[string]json.RawMessage{id: []byte(`{"health":{"failure_threshold":1,"cooldown":"1s"}}`)}); e != nil {
		t.Fatal(e)
	}
	reg, e := registry.New(manager)
	if e != nil {
		t.Fatal(e)
	}
	scheduler, e := reg.Scheduler(id)
	if e != nil {
		t.Fatal(e)
	}
	a := &sdk.Candidate{Id: "a", Metadata: map[string]string{sdk.SchedulerPriority: "10", sdk.SchedulerCapacity: "1"}}
	b := &sdk.Candidate{Id: "b", Metadata: map[string]string{sdk.SchedulerPriority: "5", sdk.SchedulerCapacity: "10"}}
	in := &sdk.SelectionInput{RouteId: "r", Request: &sdk.RequestInfo{Id: "r", Metadata: map[string]string{sdk.AffinityKey: "synthetic-session"}}, Candidates: []*sdk.Candidate{a, b}}
	selectCandidate := func() *sdk.Selection {
		t.Helper()
		v, e := scheduler.Select(context.Background(), in)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	feedback := func(v *sdk.Selection, f *sdk.Failure) {
		t.Helper()
		if e := scheduler.Report(context.Background(), &sdk.AttemptResult{RouteId: "r", RequestId: "r", CandidateId: v.CandidateId, Lease: v.Lease, Status: 200, FirstByteMs: 12, DurationMs: 30, BytesEmitted: 7, Failure: f}); e != nil {
			t.Fatal(e)
		}
	}
	v := selectCandidate()
	if v.CandidateId != "a" || len(v.Lease) != 16 {
		t.Fatal(v)
	}
	held := selectCandidate()
	if held.CandidateId != "b" {
		t.Fatal("reservation not enforced")
	}
	feedback(held, &sdk.Failure{Kind: sdk.Downstream})
	feedback(v, nil)
	feedback(v, nil)
	v = selectCandidate()
	if !v.AffinityHit || v.Reason != "sticky_hit" {
		t.Fatal(v)
	}
	feedback(v, &sdk.Failure{Kind: sdk.Transport})
	v = selectCandidate()
	if v.CandidateId != "b" || v.Reason != "sticky_escape_unhealthy" {
		t.Fatal(v)
	}
	feedback(v, &sdk.Failure{Kind: sdk.Downstream})
	old, _ := manager.Get(id)
	if e = manager.Restart(context.Background(), id); e != nil {
		t.Fatal(e)
	}
	next, _ := manager.Get(id)
	if old.PID() == next.PID() {
		t.Fatal("not restarted")
	}
	// Existing proxy must resolve the new process; local health/affinity are reset.
	v = selectCandidate()
	if v.CandidateId != "a" || v.AffinityHit {
		t.Fatal(v)
	}
	feedback(v, nil)
}

func TestAdvancedSchedulerGateway(t *testing.T) {
	a := fixture(t)
	upstream := func(label string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"label":%q,"choices":[]}`, label)
		}))
	}
	high, low := upstream("high"), upstream("low")
	defer high.Close()
	defer low.Close()
	c := cfgFor(t, high.URL, low.URL)
	c.Routes[0].Scheduler = "scheduler.priority-weight-health"
	c.Upstreams["u0"] = config.Upstream{Plugin: "upstream.http", Config: config.Object{"base_url": high.URL + "/v1"}, Metadata: map[string]string{sdk.SchedulerPriority: "10", sdk.SchedulerCapacity: "1"}}
	c.Upstreams["u1"] = config.Upstream{Plugin: "upstream.http", Config: config.Object{"base_url": low.URL + "/v1"}, Metadata: map[string]string{sdk.SchedulerPriority: "5"}}
	s := server(t, a, c, nil)
	for range 3 {
		req, _ := http.NewRequest("POST", s.URL+"/v1/chat/completions", strings.NewReader(`{"model":"test","messages":[{}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Moduapi-Affinity", "synthetic-session")
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		body := read(t, res)
		res.Body.Close()
		if res.StatusCode != 200 || !strings.Contains(body, "high") {
			t.Fatal(res.StatusCode, body)
		}
	}
	// Admit a real lease independently to occupy high priority; HTTP must escape.
	scheduler, _ := a.Registry.Scheduler("scheduler.priority-weight-health")
	held, e := scheduler.Select(context.Background(), &sdk.SelectionInput{RouteId: "hold", Request: &sdk.RequestInfo{}, Candidates: []*sdk.Candidate{{Id: "u0", Metadata: map[string]string{sdk.SchedulerCapacity: "1"}}}})
	if e != nil {
		t.Fatal(e)
	}
	if body := read(t, post(t, s.URL, "test", false)); !strings.Contains(body, "low") {
		t.Fatal(body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = scheduler.Report(ctx, &sdk.AttemptResult{RouteId: "hold", CandidateId: "u0", Lease: held.Lease, Failure: &sdk.Failure{Kind: sdk.Downstream}}); e != nil {
		t.Fatal(e)
	}
}
