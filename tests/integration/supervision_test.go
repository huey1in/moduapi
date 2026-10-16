package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/core/gateway"
	"github.com/huey1in/moduapi/core/router"
	"github.com/huey1in/moduapi/runtime/manifest"
	"github.com/huey1in/moduapi/runtime/process"
	"github.com/huey1in/moduapi/runtime/registry"
	"github.com/huey1in/moduapi/sdk"
)

func eventually(t *testing.T, pred func() bool, message string) {
	t.Helper()
	end := time.Now().Add(10 * time.Second)
	for !pred() {
		if time.Now().After(end) {
			t.Fatal(message)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func killPlugin(t *testing.T, p *process.Process) {
	t.Helper()
	child, e := os.FindProcess(p.PID())
	if e != nil {
		t.Fatal(e)
	}
	if e = child.Kill(); e != nil {
		t.Fatal(e)
	}
	wait(t, p.Done(), "process not reaped")
}
func TestBoundedSupervision(t *testing.T) {
	decl := helperManifest(t, "serve")
	o := process.DefaultRestartOptions()
	o.Initial = 20 * time.Millisecond
	o.Max = 50 * time.Millisecond
	o.CrashLimit = 3
	manager := process.NewManagerWithOptions(o)
	defer manager.Close(context.Background())
	if e := manager.Start(context.Background(), []manifest.Manifest{decl}, nil); e != nil {
		t.Fatal(e)
	}
	reg, e := registry.New(manager)
	if e != nil {
		t.Fatal(e)
	}
	scheduler, _ := reg.Scheduler(decl.ID)
	for n := 0; n < 3; n++ {
		p, e := manager.Get(decl.ID)
		if e != nil {
			t.Fatal(e)
		}
		killPlugin(t, p)
		if n < 2 {
			eventually(t, func() bool { next, e := manager.Get(decl.ID); return e == nil && next.PID() != p.PID() }, "no automatic replacement")
			if _, e := scheduler.Select(context.Background(), &sdk.SelectionInput{}); e != nil {
				t.Fatal("old proxy not rebound", e)
			}
		}
	}
	eventually(t, func() bool { d, _ := manager.Diagnostics(decl.ID); return d.Failed }, "crash-loop protection not activated")
	d, _ := manager.Diagnostics(decl.ID)
	if d.Ready || d.Restarts != 2 {
		t.Fatal(d)
	}
	cfg := config.Config{Routes: []router.Route{{ID: "r", Protocol: "test", Model: "*", Scheduler: decl.ID, Upstreams: []string{"a"}}}, Upstreams: map[string]config.Upstream{"a": {Plugin: "unused"}}}
	cfg.Defaults()
	g, e := gateway.New(cfg, reg, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	for path, want := range map[string]int{"/healthz": 200, "/readyz": 503} {
		rr := httptest.NewRecorder()
		g.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != want {
			t.Fatal(path, rr.Code)
		}
	}
	if e := manager.Restart(context.Background(), decl.ID); e != nil {
		t.Fatal(e)
	}
	d, _ = manager.Diagnostics(decl.ID)
	if !d.Ready || d.Failed || d.Restarts != 0 {
		t.Fatal("intentional restart did not reset protection", d)
	}
	p, _ := manager.Get(decl.ID)
	p.Stop(context.Background())
	time.Sleep(100 * time.Millisecond)
	d, _ = manager.Diagnostics(decl.ID)
	if d.Failed || d.Restarts != 0 || d.Ready {
		t.Fatal("intentional stop counted as crash", d)
	}
	if !strings.Contains(p.StderrTail(), "draining") {
		t.Fatal("safe SDK diagnostics not retained")
	}
}
func TestReplacementStartupFailureStops(t *testing.T) {
	decl := helperManifest(t, "serve")
	o := process.DefaultRestartOptions()
	o.Initial = 300 * time.Millisecond
	o.Max = time.Second
	manager := process.NewManagerWithOptions(o)
	defer manager.Close(context.Background())
	if e := manager.Start(context.Background(), []manifest.Manifest{decl}, nil); e != nil {
		t.Fatal(e)
	}
	p, _ := manager.Get(decl.ID)
	killPlugin(t, p)
	binary := filepath.Join(filepath.Dir(decl.Path), decl.Runtime.Command)
	if e := os.Rename(binary, binary+".unavailable"); e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool { d, _ := manager.Diagnostics(decl.ID); return d.Failed }, "startup failure did not stop supervisor")
	d, _ := manager.Diagnostics(decl.ID)
	if d.Restarts != 0 || d.Ready {
		t.Fatal(d)
	}
}

func TestCloseCancelsBackoff(t *testing.T) {
	decl := helperManifest(t, "serve")
	o := process.DefaultRestartOptions()
	o.Initial = 30 * time.Second
	o.Max = 30 * time.Second
	manager := process.NewManagerWithOptions(o)
	defer manager.Close(context.Background())
	if e := manager.Start(context.Background(), []manifest.Manifest{decl}, nil); e != nil {
		t.Fatal(e)
	}
	p, _ := manager.Get(decl.ID)
	killPlugin(t, p)
	started := time.Now()
	manager.Close(context.Background())
	if time.Since(started) > 3*time.Second {
		t.Fatal("shutdown waited for restart backoff")
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("dead child not reaped")
	}
}
