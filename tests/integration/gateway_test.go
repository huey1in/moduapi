package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/core/gateway"
	"github.com/huey1in/moduapi/core/lifecycle"
	"github.com/huey1in/moduapi/core/retry"
	"github.com/huey1in/moduapi/core/router"
	"github.com/huey1in/moduapi/internal/app"
	"github.com/huey1in/moduapi/internal/buildutil"
	"github.com/huey1in/moduapi/internal/mockupstream"
	"github.com/huey1in/moduapi/sdk"
)

var bundles string

func TestMain(m *testing.M) {
	if os.Getenv("MODUAPI_PLUGIN_TOKEN") != "" {
		runHelper()
		return
	}
	root, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		log.Fatal(e)
	}
	tmp, e := os.MkdirTemp("", "moduapi-integration-")
	if e != nil {
		log.Fatal(e)
	}
	bundles = filepath.Join(tmp, "plugins")
	if e = buildutil.Build(root, bundles, raceEnabled); e != nil {
		fmt.Fprintln(os.Stderr, e)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}
	code := m.Run()
	// SDK processes observe parent pipe EOF asynchronously. On Windows/race
	// builds their final exit can briefly retain an executable file handle.
	for attempt := 0; attempt < 100; attempt++ {
		e = os.RemoveAll(tmp)
		if e == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "plugin bundle cleanup failed:", e)
		code = 1
	}
	os.Exit(code)
}
func cfgFor(t *testing.T, urls ...string) config.Config {
	t.Helper()
	c := config.Config{Plugins: config.Plugins{Directory: bundles}, Upstreams: map[string]config.Upstream{}}
	r := router.Route{ID: t.Name(), Protocol: "openai.chat", Model: "*", Scheduler: "scheduler.single"}
	for i, u := range urls {
		id := fmt.Sprintf("u%d", i)
		r.Upstreams = append(r.Upstreams, id)
		c.Upstreams[id] = config.Upstream{Plugin: "upstream.http", Config: config.Object{"base_url": u + "/v1"}}
	}
	c.Routes = []router.Route{r}
	c.Defaults()
	return c
}
func fixture(t *testing.T) *app.App {
	t.Helper()
	a, e := app.Start(context.Background(), cfgFor(t, "http://127.0.0.1:1"), nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		list := a.Manager.List()
		a.Manager.Close(context.Background())
		for _, p := range list {
			if e := p.ExitError(); e != nil {
				t.Errorf("plugin %s exit: %v", p.Manifest.ID, e)
			}
		}
	})
	return a
}
func server(t *testing.T, a *app.App, c config.Config, hooks *lifecycle.Bus) *httptest.Server {
	return serverWithPolicy(t, a, c, hooks, nil)
}

type alwaysRetry struct{}

func (alwaysRetry) Decide(context.Context, retry.State) retry.Decision {
	return retry.Decision{Retry: true}
}
func serverWithPolicy(t *testing.T, a *app.App, c config.Config, hooks *lifecycle.Bus, policy retry.Policy) *httptest.Server {
	t.Helper()
	g, e := gateway.New(c, a.Registry, hooks, policy)
	if e != nil {
		t.Fatal(e)
	}
	s := httptest.NewUnstartedServer(g)
	s.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.Start()
	t.Cleanup(s.Close)
	return s
}
func post(t *testing.T, url, model string, stream bool) *http.Response {
	t.Helper()
	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}],"stream":%t}`, model, stream)
	r, e := http.Post(url+"/v1/chat/completions", "application/json", strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Body.Close() })
	return r
}
func read(t *testing.T, r *http.Response) string {
	t.Helper()
	b, e := io.ReadAll(r.Body)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func wait(t *testing.T, ch <-chan struct{}, reason string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal(reason)
	}
}
func TestVerticalSlice(t *testing.T) {
	a := fixture(t)
	t.Run("JSON lifecycle usage and headers", func(t *testing.T) {
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
				t.Error("ingress credentials leaked")
			}
			w.Header().Set("Connection", "X-Private")
			w.Header().Set("X-Private", "must disappear")
			mockupstream.Handler(w, r)
		}))
		defer up.Close()
		var mu sync.Mutex
		var stages []lifecycle.Stage
		var usage *sdk.UsageEvent
		finished := make(chan struct{})
		h := lifecycle.New(lifecycle.HookFunc(func(ctx context.Context, e lifecycle.Event) error {
			mu.Lock()
			defer mu.Unlock()
			stages = append(stages, e.Stage)
			if e.Stage == lifecycle.Complete {
				usage = e.Usage
				close(finished)
			}
			return nil
		}))
		g := server(t, a, cfgFor(t, up.URL), h)
		req, _ := http.NewRequest("POST", g.URL+"/v1/chat/completions", strings.NewReader(`{"model":"mock","messages":[{"role":"user","content":"hi"}],"extension":{"preserve":true}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer downstream-only")
		req.Header.Set("Cookie", "downstream=cookie")
		r, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		b := read(t, r)
		wait(t, finished, "completion missing")
		if r.StatusCode != 200 || !strings.Contains(b, "Hello from ModuAPI mock") || r.Header.Get("X-Private") != "" || r.Header.Get("X-Request-ID") == "" {
			t.Fatalf("status=%d body=%s headers=%v", r.StatusCode, b, r.Header)
		}
		mu.Lock()
		defer mu.Unlock()
		want := []lifecycle.Stage{lifecycle.Ingress, lifecycle.Decoded, lifecycle.Routed, lifecycle.Selected, lifecycle.AttemptEnd, lifecycle.Complete}
		if !reflect.DeepEqual(stages, want) || usage.InputTokens != 3 || usage.OutputTokens != 5 || usage.Partial {
			t.Fatalf("stages=%v usage=%+v", stages, usage)
		}
	})
	t.Run("SSE arrives before upstream finishes", func(t *testing.T) {
		release := make(chan struct{})
		upDone := make(chan struct{})
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(upDone)
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"first\"}}]}\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-release:
			}
			_, _ = io.WriteString(w, "data: {\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":9}}\n\ndata: [DONE]\n\n")
		}))
		defer up.Close()
		defer func() {
			select {
			case <-release:
			default:
				close(release)
			}
		}()
		g := server(t, a, cfgFor(t, up.URL), nil)
		r := post(t, g.URL, "mock", true)
		scanner := bufio.NewReader(r.Body)
		line, e := scanner.ReadString('\n')
		if e != nil || !strings.Contains(line, "first") {
			t.Fatalf("%q %v", line, e)
		}
		select {
		case <-upDone:
			t.Fatal("stream buffered until completion")
		default:
		}
		close(release)
		rest, e := io.ReadAll(scanner)
		if e != nil || !strings.Contains(string(rest), "[DONE]") {
			t.Fatalf("%s %v", rest, e)
		}
	})
	t.Run("round robin and concurrent requests", func(t *testing.T) {
		var ca, cb atomic.Int64
		makeUp := func(count *atomic.Int64) *httptest.Server {
			return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); mockupstream.Handler(w, r) }))
		}
		ua, ub := makeUp(&ca), makeUp(&cb)
		defer ua.Close()
		defer ub.Close()
		c := cfgFor(t, ua.URL, ub.URL)
		c.Routes[0].Scheduler = "scheduler.roundrobin"
		g := server(t, a, c, nil)
		for i := range 6 {
			r := post(t, g.URL, "mock", false)
			_ = read(t, r)
			if ca.Load() != int64((i+2)/2) || cb.Load() != int64((i+1)/2) {
				t.Fatalf("not alternating: %d %d", ca.Load(), cb.Load())
			}
		}
		var wg sync.WaitGroup
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				body := bytes.NewBufferString(`{"model":"mock","messages":[{"role":"user","content":"hi"}]}`)
				r, e := http.Post(g.URL+"/v1/chat/completions", "application/json", body)
				if e != nil {
					t.Error(e)
					return
				}
				defer r.Body.Close()
				_, e = io.Copy(io.Discard, r.Body)
				if e != nil || r.StatusCode != 200 {
					t.Errorf("%v status %d", e, r.StatusCode)
				}
			}()
		}
		wg.Wait()
		if ca.Load() != 13 || cb.Load() != 13 {
			t.Fatalf("imbalanced: %d %d", ca.Load(), cb.Load())
		}
	})
	t.Run("protocol terminal completes without socket EOF", func(t *testing.T) {
		cancelled := make(chan struct{})
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[]}\n\ndata: [DONE]\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(cancelled)
		}))
		defer up.Close()
		g := server(t, a, cfgFor(t, up.URL), nil)
		r := post(t, g.URL, "mock", true)
		if !strings.Contains(read(t, r), "[DONE]") {
			t.Fatal("terminal missing")
		}
		wait(t, cancelled, "terminal did not close upstream RPC")
	})
	t.Run("upstream status and retry opt-in", func(t *testing.T) {
		for _, status := range []int{400, 401, 429, 500, 503} {
			t.Run(fmt.Sprint(status), func(t *testing.T) {
				var calls atomic.Int64
				bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"error":{"message":"retained upstream response"}}`)
				}))
				defer bad.Close()
				good := httptest.NewServer(http.HandlerFunc(mockupstream.Handler))
				defer good.Close()
				c := cfgFor(t, bad.URL, good.URL)
				g := server(t, a, c, nil)
				r := post(t, g.URL, "mock", false)
				if r.StatusCode != status || !strings.Contains(read(t, r), "retained upstream response") || calls.Load() != 1 {
					t.Fatal("default request retried or status lost")
				}
				c.Routes[0].Retry = router.Retry{MaxAttempts: 2, AllowReplay: true}
				g = server(t, a, c, nil)
				r = post(t, g.URL, "mock", false)
				b := read(t, r)
				want := status
				if status == 429 || status == 503 {
					want = 200
				}
				if r.StatusCode != want {
					t.Fatalf("status=%d want=%d %s", r.StatusCode, want, b)
				}
			})
		}
	})
	t.Run("model mapping preserves original and opaque fields", func(t *testing.T) {
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			var m map[string]json.RawMessage
			_ = json.Unmarshal(b, &m)
			if string(m["model"]) != `"mapped"` || string(m["custom"]) != `{"x":42}` {
				t.Errorf("mapping lost payload: %s", b)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"choices":[],"usage":{"prompt_tokens":1}}`)
		}))
		defer up.Close()
		c := cfgFor(t, up.URL)
		u := c.Upstreams["u0"]
		u.ProtocolConfig = config.Object{"model_mapping": map[string]any{"alias": "mapped"}}
		c.Upstreams["u0"] = u
		g := server(t, a, c, nil)
		r, e := http.Post(g.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"alias","messages":[{"role":"user","content":"hi"}],"custom":{"x":42}}`))
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		_ = read(t, r)
		if r.StatusCode != 200 {
			t.Fatal(r.StatusCode)
		}
	})
	t.Run("validation and body limit", func(t *testing.T) {
		up := httptest.NewServer(http.HandlerFunc(mockupstream.Handler))
		defer up.Close()
		c := cfgFor(t, up.URL)
		c.Server.MaxBodyBytes = 256
		g := server(t, a, c, nil)
		for _, tc := range []struct {
			body   string
			status int
		}{{`{`, 400}, {`{"model":"m","model":"different","messages":[{}]}`, 400}, {`{"model":"m","messages":[]}`, 400}, {strings.Repeat("x", 257), 413}} {
			r, e := http.Post(g.URL+"/v1/chat/completions", "application/json", strings.NewReader(tc.body))
			if e != nil {
				t.Fatal(e)
			}
			_ = read(t, r)
			r.Body.Close()
			if r.StatusCode != tc.status {
				t.Fatalf("%q: %d", tc.body, r.StatusCode)
			}
		}
	})
	t.Run("first byte timeout cancels upstream", func(t *testing.T) {
		for _, sendHeaders := range []bool{false, true} {
			t.Run(fmt.Sprint(sendHeaders), func(t *testing.T) {
				cancelled := make(chan struct{})
				up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					if sendHeaders {
						w.Header().Set("Content-Type", "text/event-stream")
						w.WriteHeader(200)
						w.(http.Flusher).Flush()
					}
					<-r.Context().Done()
					close(cancelled)
				}))
				defer up.Close()
				c := cfgFor(t, up.URL)
				c.Server.FirstByteTimeout = config.Duration(150 * time.Millisecond)
				g := server(t, a, c, nil)
				r := post(t, g.URL, "mock", sendHeaders)
				b := read(t, r)
				if r.StatusCode != 504 || !strings.Contains(b, "first_byte_timeout") {
					t.Fatalf("%d %s", r.StatusCode, b)
				}
				wait(t, cancelled, "upstream not cancelled")
			})
		}
	})
	t.Run("request deadline reaches plugin and upstream", func(t *testing.T) {
		cancelled := make(chan struct{})
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			<-r.Context().Done()
			close(cancelled)
		}))
		defer up.Close()
		c := cfgFor(t, up.URL)
		c.Server.RequestTimeout = config.Duration(200 * time.Millisecond)
		g := server(t, a, c, nil)
		r := post(t, g.URL, "mock", false)
		if r.StatusCode != 504 {
			t.Fatal(r.StatusCode, read(t, r))
		}
		wait(t, cancelled, "deadline did not cancel transport")
	})
	t.Run("downstream cancellation reaches upstream", func(t *testing.T) {
		started, cancelled := make(chan struct{}), make(chan struct{})
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			close(started)
			<-r.Context().Done()
			close(cancelled)
		}))
		defer up.Close()
		g := server(t, a, cfgFor(t, up.URL), nil)
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "POST", g.URL+"/v1/chat/completions", strings.NewReader(`{"model":"mock","messages":[{}]}`))
		req.Header.Set("Content-Type", "application/json")
		clientDone := make(chan struct{})
		go func() {
			defer close(clientDone)
			r, e := http.DefaultClient.Do(req)
			if e == nil {
				r.Body.Close()
			}
		}()
		wait(t, started, "upstream not started")
		cancel()
		wait(t, clientDone, "client did not cancel")
		wait(t, cancelled, "plugin leaked request")
	})
	t.Run("no failover after partial output", func(t *testing.T) {
		var fallback atomic.Int64
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
			w.(http.Flusher).Flush()
		}))
		defer bad.Close()
		good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fallback.Add(1); mockupstream.Handler(w, r) }))
		defer good.Close()
		c := cfgFor(t, bad.URL, good.URL)
		c.Routes[0].Retry = router.Retry{MaxAttempts: 2, AllowReplay: true}
		g := serverWithPolicy(t, a, c, nil, alwaysRetry{})
		r := post(t, g.URL, "mock", true)
		b, e := io.ReadAll(r.Body)
		if e == nil || !strings.Contains(string(b), "partial") || strings.Contains(string(b), "[DONE]") || fallback.Load() != 0 {
			t.Fatalf("body=%s error=%v fallback=%d", b, e, fallback.Load())
		}
	})
	t.Run("idle timeout aborts partial stream", func(t *testing.T) {
		cancelled := make(chan struct{})
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[]}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(cancelled)
		}))
		defer up.Close()
		c := cfgFor(t, up.URL)
		c.Server.IdleTimeout = config.Duration(150 * time.Millisecond)
		g := server(t, a, c, nil)
		r := post(t, g.URL, "mock", true)
		_, e := io.ReadAll(r.Body)
		if e == nil {
			t.Fatal("idle failure appeared successful")
		}
		wait(t, cancelled, "idle timeout left upstream alive")
	})
	t.Run("hook rejection propagates without contacting upstream", func(t *testing.T) {
		var count atomic.Int64
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); mockupstream.Handler(w, r) }))
		defer up.Close()
		hooks := lifecycle.New(lifecycle.HookFunc(func(ctx context.Context, e lifecycle.Event) error {
			if e.Stage == lifecycle.Routed {
				return sdk.Fail(sdk.Local, "policy_rejection", "test policy rejected request", 403)
			}
			return nil
		}))
		g := server(t, a, cfgFor(t, up.URL), hooks)
		r := post(t, g.URL, "mock", false)
		if r.StatusCode != 403 || count.Load() != 0 || !strings.Contains(read(t, r), "policy_rejection") {
			t.Fatal("hook failure swallowed")
		}
	})
	t.Run("graceful plugin drain keeps active stream alive", func(t *testing.T) {
		release := make(chan struct{})
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[]}\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-release:
			}
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		}))
		defer up.Close()
		defer func() {
			select {
			case <-release:
			default:
				close(release)
			}
		}()
		g := server(t, a, cfgFor(t, up.URL), nil)
		r := post(t, g.URL, "mock", true)
		reader := bufio.NewReader(r.Body)
		if _, e := reader.ReadString('\n'); e != nil {
			t.Fatal(e)
		}
		p, e := a.Manager.Get("upstream.http")
		if e != nil {
			t.Fatal(e)
		}
		stopped := make(chan struct{})
		go func() { p.Stop(context.Background()); close(stopped) }()
		select {
		case <-stopped:
			t.Fatal("plugin stopped before active stream drained")
		case <-time.After(40 * time.Millisecond):
		}
		if p.Ready() {
			t.Fatal("draining process is still ready")
		}
		close(release)
		b, e := io.ReadAll(reader)
		if e != nil || !strings.Contains(string(b), "[DONE]") {
			t.Fatalf("stream was cut during drain: %s %v", b, e)
		}
		wait(t, stopped, "graceful plugin did not stop")
		if e = p.ExitError(); e != nil {
			t.Fatal(e)
		}
		if e = a.Manager.Restart(context.Background(), "upstream.http"); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("plugin crash preserves kernel health and restart", func(t *testing.T) {
		up := httptest.NewServer(http.HandlerFunc(mockupstream.Handler))
		defer up.Close()
		g := server(t, a, cfgFor(t, up.URL), nil)
		p, e := a.Manager.Get("protocol.openai")
		if e != nil {
			t.Fatal(e)
		}
		child, e := os.FindProcess(p.PID())
		if e != nil {
			t.Fatal(e)
		}
		if e = child.Kill(); e != nil {
			t.Fatal(e)
		}
		wait(t, p.Done(), "plugin exit not detected")
		for path, status := range map[string]int{"/healthz": 200, "/readyz": 503} {
			r, e := http.Get(g.URL + path)
			if e != nil {
				t.Fatal(e)
			}
			r.Body.Close()
			if r.StatusCode != status {
				t.Fatalf("%s: %d", path, r.StatusCode)
			}
		}
		r := post(t, g.URL, "mock", false)
		if r.StatusCode != 502 {
			t.Fatal(r.StatusCode)
		}
		if e = a.Manager.Restart(context.Background(), "protocol.openai"); e != nil {
			t.Fatal(e)
		}
		r = post(t, g.URL, "mock", false)
		if r.StatusCode != 200 {
			t.Fatal(r.StatusCode, read(t, r))
		}
		_ = read(t, r)
	})
}
