// bench-e2e is an audit harness, not a production transport or server mode.
// It uses the unchanged app/runtime and three real official plugin processes.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/core/gateway"
	"github.com/huey1in/moduapi/core/router"
	"github.com/huey1in/moduapi/internal/app"
	"github.com/huey1in/moduapi/internal/buildutil"
	"github.com/huey1in/moduapi/sdk/plugin"
	"github.com/huey1in/moduapi/validation/placement"
)

type options struct {
	Role, Bundles, Scheduler, Upstream, Out, Concurrency, Topologies, Scenarios, Scenario string
	BodyBytes                                                                             int
	Soak                                                                                  time.Duration
	Duration, Warmup                                                                      time.Duration
	Repeats                                                                               int
	Build                                                                                 bool
}
type hello struct {
	URL  string
	PIDs []int
}
type stats struct {
	Goroutines                   int
	Active, Cancelled, MaxActive int64
	RSS, Descriptors             uint64
	PIDs                         []int
}

var active, cancelled, maxActive atomic.Int64

type child struct {
	hello
	cmd   *exec.Cmd
	input io.WriteCloser
	done  chan error
}
type sample struct {
	TotalNS     int64 `json:"total_ns"`
	FirstNS     int64 `json:"first_ns"`
	WorkNS      int64 `json:"work_ns"`
	WorkFirstNS int64 `json:"work_first_ns"`
}
type distribution struct{ P50, P95, P99 float64 }

var workload options

func snapshot(url string) stats {
	c := http.Client{Timeout: time.Second}
	r, e := c.Get(url + "/_bench/stats")
	if e != nil {
		return stats{Goroutines: -1}
	}
	defer r.Body.Close()
	var s stats
	_ = json.NewDecoder(r.Body).Decode(&s)
	return s
}

type result struct {
	Topology, Mode, Scenario                                               string
	BodyBytes                                                              int
	Cancellations                                                          int
	ResourceSamples                                                        []stats
	RecoveryMS                                                             float64
	WarmupMS                                                               float64
	MockFinal                                                              stats
	StartupMS                                                              float64
	Concurrency, Repeat, Requests, Errors                                  int
	Seconds, RPS                                                           float64
	TotalMS, FirstMS, ResidualMS, FirstResidualMS                          distribution
	CPUSeconds, CPUCores                                                   float64
	PeakRSSBytes                                                           uint64
	PeakDescriptors                                                        uint64
	DescriptorKind                                                         string
	Processes, HostGoroutinesIdle, HostGoroutinesPeak, HostGoroutinesAfter int
	Samples                                                                []sample `json:"-"`
}
type output struct {
	Baseline                string
	Experiment              string
	Go, OS, Arch, CPU       string
	LogicalCPUs, GOMAXPROCS int
	StartedUTC              string
	Options                 options
	Results                 []result
}

func main() {
	var o options
	flag.StringVar(&o.Role, "role", "run", "run, mock or gateway (internal child modes)")
	flag.StringVar(&o.Bundles, "bundles", "dist/plugins", "official plugin bundles")
	flag.StringVar(&o.Scheduler, "scheduler", "scheduler.single", "child scheduler")
	flag.StringVar(&o.Upstream, "upstream", "", "child mock URL")
	flag.StringVar(&o.Out, "out", "docs/validation/phase-3a-"+runtime.GOOS+".json", "raw result file")
	flag.StringVar(&o.Concurrency, "concurrency", "1,16,64", "closed-loop worker counts")
	flag.DurationVar(&o.Duration, "duration", 5*time.Second, "measurement per case")
	flag.DurationVar(&o.Warmup, "warmup", time.Second, "warmup per case")
	flag.IntVar(&o.Repeats, "repeats", 3, "independent repetitions; fresh processes per case")
	flag.BoolVar(&o.Build, "build", true, "build official bundles before running")
	flag.StringVar(&o.Topologies, "topologies", "direct,split,bundle,local", "execution placements")
	flag.StringVar(&o.Scenarios, "scenarios", "json1k,json64k,json1m,sse,slow,cancel,crash", "workloads")
	flag.StringVar(&o.Scenario, "scenario", "json1k", "internal child scenario")
	flag.IntVar(&o.BodyBytes, "body-bytes", 1024, "internal request content bytes")
	flag.DurationVar(&o.Soak, "soak", 0, "mixed stability duration per placement; separate from performance cases")
	flag.Parse()
	var err error
	if o.Role == "run" {
		err = run(o)
	} else {
		err = serve(o)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(o options) error {
	if o.Role == "bundle-plugin" {
		return plugin.Run(placement.Definition())
	}
	var a *app.App
	var local *placement.Local
	var handler http.Handler = http.HandlerFunc(mock)
	pids := []int{os.Getpid()}
	if o.Role == "gateway" {
		// Construct a bundle directory with exactly the requested three packages.
		bundleDir, err := os.MkdirTemp("", "moduapi-e2e-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(bundleDir)
		ids := []string{"protocol.openai", "scheduler.single", "upstream.http"}
		if o.Scheduler == "bundle" {
			ids = []string{"validation.bundle"}
		}
		for _, id := range ids {
			dir := filepath.Join(bundleDir, id)
			if err = os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			for _, name := range []string{"manifest.yaml", "plugin" + buildutil.Suffix()} {
				b, e := os.ReadFile(filepath.Join(o.Bundles, id, name))
				if e != nil {
					return e
				}
				if e = os.WriteFile(filepath.Join(dir, name), b, 0755); e != nil {
					return e
				}
			}
		}
		schedulerID, upstreamID := "scheduler.single", "upstream.http"
		if o.Scheduler == "bundle" {
			schedulerID, upstreamID = "validation.bundle", "validation.bundle"
		}
		if o.Scheduler == "local" {
			schedulerID, upstreamID = "local", "local"
		}
		c := config.Config{Plugins: config.Plugins{Directory: bundleDir}, Routes: []router.Route{{ID: "bench", Protocol: "openai.chat", Model: "*", Scheduler: schedulerID, Upstreams: []string{"a", "b"}}}, Upstreams: map[string]config.Upstream{}}
		// Same two candidates and target for all policies; no affinity/capacity or retry.
		for _, id := range []string{"a", "b"} {
			c.Upstreams[id] = config.Upstream{Plugin: upstreamID, Config: config.Object{"base_url": o.Upstream + "/v1"}}
		}
		if o.Scheduler == "local" {
			local = placement.New()
			defer local.Close()
			g, e := gateway.New(c, local, nil, nil)
			if e != nil {
				return e
			}
			handler = g
		} else {
			a, err = app.Start(context.Background(), c, nil)
			if err != nil {
				return err
			}
			defer a.Manager.Close(context.Background())
			for _, p := range a.Manager.List() {
				pids = append(pids, p.PID())
			}
			handler = a.Gateway
		}
	} else if o.Role != "mock" {
		return fmt.Errorf("unknown role")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_bench/stats" {
			if a != nil {
				pids = []int{os.Getpid()}
				for _, p := range a.Manager.List() {
					pids = append(pids, p.PID())
				}
			}
			snapshot := stats{Goroutines: runtime.NumGoroutine(), Active: active.Load(), Cancelled: cancelled.Load(), MaxActive: maxActive.Load(), PIDs: pids}
			if v, e := readGroup(pids); e == nil {
				snapshot.RSS = v.RSS
				snapshot.Descriptors = v.Descriptors
			}
			_ = json.NewEncoder(w).Encode(snapshot)
			return
		}
		if r.URL.Path == "/_bench/crash" && a != nil {
			for _, p := range a.Manager.List() {
				if p.Manifest.ID == "upstream.http" || p.Manifest.ID == "validation.bundle" {
					proc, _ := os.FindProcess(p.PID())
					_ = proc.Kill()
					break
				}
			}
			w.WriteHeader(204)
			return
		}
		handler.ServeHTTP(w, r)
	}), ReadHeaderTimeout: 5 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(ln) }()
	if err = json.NewEncoder(os.Stdout).Encode(hello{"http://" + ln.Addr().String(), pids}); err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, os.Stdin) // parent owns the helper lifetime
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		_ = server.Close()
	}
	<-done
	return err
}

func mock(w http.ResponseWriter, r *http.Request) {
	n := active.Add(1)
	defer active.Add(-1)
	for old := maxActive.Load(); n > old; old = maxActive.Load() {
		if maxActive.CompareAndSwap(old, n) {
			break
		}
	}
	started := nanoNow()
	if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Stream   bool   `json:"stream"`
		Scenario string `json:"validation_scenario"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req) != nil {
		http.Error(w, "invalid", 400)
		return
	}
	// Fixed intentional work, measured rather than assuming timer accuracy.
	time.Sleep(time.Millisecond)
	if !req.Stream {
		w.Header().Set("Content-Type", "application/json")
		work := nanoNow() - started
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "bench", "model": "mock", "choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": strings.Repeat("x", 1024)}}}, "usage": map[string]int{"prompt_tokens": 16, "completion_tokens": 32}, "bench_work_ns": work, "bench_first_ns": work})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	first := nanoNow() - started
	chunks := 4
	if req.Scenario == "slow" {
		chunks = 32768
	} // 8MiB; exceeds socket/window buffering
	if req.Scenario == "cancel" || req.Scenario == "soak" {
		chunks = 1 << 30
	}
	for i := 0; i < chunks; i++ {
		if i > 0 && req.Scenario != "slow" {
			select {
			case <-r.Context().Done():
				cancelled.Add(1)
				return
			case <-time.After(time.Millisecond):
			}
		}
		b, _ := json.Marshal(map[string]any{"id": "bench", "choices": []any{map[string]any{"delta": map[string]string{"content": strings.Repeat("x", 256)}}}, "bench_first_ns": first})
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			cancelled.Add(1)
			return
		}
		w.(http.Flusher).Flush()
	}
	b, _ := json.Marshal(map[string]any{"choices": []any{}, "usage": map[string]int{"prompt_tokens": 16, "completion_tokens": 32}, "bench_work_ns": nanoNow() - started, "bench_first_ns": first})
	_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
	w.(http.Flusher).Flush()
}

func start(args ...string) (*child, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	c := &child{cmd: exec.Command(exe, args...), done: make(chan error, 1)}
	c.cmd.Stderr = os.Stderr
	c.input, err = c.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := c.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = c.cmd.Start(); err != nil {
		return nil, err
	}
	go func() { c.done <- c.cmd.Wait() }()
	ready := make(chan error, 1)
	go func() { ready <- json.NewDecoder(io.LimitReader(out, 4096)).Decode(&c.hello) }()
	select {
	case err = <-ready:
	case <-time.After(30 * time.Second):
		err = fmt.Errorf("child startup timed out")
	}
	if err != nil {
		c.stop()
		return nil, err
	}
	return c, nil
}
func (c *child) stop() {
	_ = c.input.Close()
	select {
	case err := <-c.done:
		if err != nil {
			fmt.Fprintln(os.Stderr, "helper exit:", err)
		}
	case <-time.After(15 * time.Second):
		_ = c.cmd.Process.Kill()
		<-c.done
	}
}
func hostStats(url string) int {
	client := http.Client{Timeout: time.Second}
	r, err := client.Get(url + "/_bench/stats")
	if err != nil {
		return -1
	}
	defer r.Body.Close()
	var s stats
	if json.NewDecoder(r.Body).Decode(&s) != nil {
		return -1
	}
	return s.Goroutines
}
func run(o options) error {
	if o.Duration <= 0 || o.Warmup < 0 || o.Repeats < 1 {
		return fmt.Errorf("invalid measurement options")
	}
	var cs []int
	for _, v := range strings.Split(o.Concurrency, ",") {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 {
			return fmt.Errorf("invalid concurrency")
		}
		cs = append(cs, n)
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	o.Bundles, err = filepath.Abs(o.Bundles)
	if err != nil {
		return err
	}
	if o.Build {
		if err = buildutil.Build(root, o.Bundles, false); err != nil {
			return err
		}
	}
	// Audit-only multi-service executable uses this same helper binary.
	bundleDir := filepath.Join(o.Bundles, "validation.bundle")
	if err = os.MkdirAll(bundleDir, 0755); err != nil {
		return err
	}
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	binary, e := os.ReadFile(executable)
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(bundleDir, "plugin"+buildutil.Suffix()), binary, 0755); e != nil {
		return e
	}
	manifest := `{"api_version":1,"id":"validation.bundle","name":"Validation bundle","version":"0.1.0","runtime":{"type":"process","command":"plugin` + buildutil.Suffix() + `","args":["-role=bundle-plugin"]},"provides":["protocol","scheduler","upstream"],"requires":{"gateway":">=0.1.0 <1.0.0"},"capabilities":["streaming","usage","affinity"],"permissions":{"network":["127.0.0.1"]},"config_schema":{"type":"object"}}`
	if e = os.WriteFile(filepath.Join(bundleDir, "manifest.yaml"), []byte(manifest), 0644); e != nil {
		return e
	}
	data := output{Baseline: "71d69e6", Experiment: "phase-3a-placement-validation-working-tree", Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, CPU: cpuName(), LogicalCPUs: runtime.NumCPU(), GOMAXPROCS: runtime.GOMAXPROCS(0), StartedUTC: time.Now().UTC().Format(time.RFC3339), Options: o}
	topologies := strings.Split(o.Topologies, ",")
	scenarios := strings.Split(o.Scenarios, ",")
	if o.Soak > 0 {
		seen := false
		for _, scenario := range scenarios {
			if scenario == "soak" {
				seen = true
				break
			}
		}
		if !seen {
			scenarios = append(scenarios, "soak")
		}
	}
	for rep := 0; rep < o.Repeats; rep++ {
		for _, c := range cs {
			for _, scenario := range scenarios {
				for j := range topologies {
					top := topologies[(j+rep)%len(topologies)]
					if top == "direct" && scenario == "crash" {
						continue
					}
					current := o
					current.Scenario = scenario
					mode := "JSON"
					if scenario == "sse" || scenario == "slow" || scenario == "cancel" || scenario == "soak" {
						mode = "SSE"
					}
					current.BodyBytes = 1024
					if scenario == "json64k" {
						current.BodyBytes = 64 << 10
					}
					if scenario == "json1m" {
						current.BodyBytes = 1 << 20
					}
					if scenario == "soak" && o.Soak > 0 {
						current.Duration = o.Soak
					}
					r, e := measure(current, top, mode, c, rep+1)
					if e != nil {
						return e
					}
					data.Results = append(data.Results, r)
					b, e := json.MarshalIndent(data, "", "  ")
					if e != nil {
						return e
					}
					if e = os.MkdirAll(filepath.Dir(o.Out), 0755); e != nil {
						return e
					}
					if e = os.WriteFile(o.Out, append(b, '\n'), 0644); e != nil {
						return e
					}
					fmt.Printf("%s %s c=%d repeat=%d n=%d cancels=%d errors=%d rps=%.1f p50=%.3fms p99=%.3fms RSS=%.1fMiB recovery=%.1fms\n", top, scenario, c, rep+1, r.Requests, r.Cancellations, r.Errors, r.RPS, r.TotalMS.P50, r.TotalMS.P99, float64(r.PeakRSSBytes)/(1<<20), r.RecoveryMS)
				}
			}
		}
	}
	return nil
}

func measure(o options, top, mode string, workers, rep int) (result, error) {
	r := result{Topology: top, Mode: mode, Scenario: o.Scenario, BodyBytes: o.BodyBytes, Concurrency: workers, Repeat: rep}
	bootStarted := nanoNow()
	mockChild, err := start("-role", "mock")
	if err != nil {
		return r, err
	}
	defer mockChild.stop()
	target := mockChild
	if top != "direct" {
		target, err = start("-role", "gateway", "-bundles", o.Bundles, "-scheduler", top, "-upstream", mockChild.URL)
		if err != nil {
			return r, err
		}
		defer target.stop()
	}
	r.StartupMS = float64(nanoNow()-bootStarted) / 1e6
	tr := &http.Transport{Proxy: nil, MaxIdleConns: workers + 4, MaxIdleConnsPerHost: workers + 4, MaxConnsPerHost: workers + 4, DisableCompression: true}
	client := &http.Client{Transport: tr, Timeout: 30 * time.Second}
	defer tr.CloseIdleConnections()
	stream := mode == "SSE"
	workload = o // each helper is separate; load workers only read this immutable value
	_, warmErrors := load(client, target.URL, stream, workers, o.Warmup)
	if warmErrors != 0 {
		return r, fmt.Errorf("warmup had %d errors", warmErrors)
	}
	r.HostGoroutinesIdle = hostStats(target.URL)
	if o.Scenario == "crash" && top != "local" {
		started := nanoNow()
		resp, e := client.Post(target.URL+"/_bench/crash", "application/json", nil)
		if e != nil {
			return r, e
		}
		resp.Body.Close()
		deadline := time.Now().Add(10 * time.Second)
		for {
			resp, e = client.Get(target.URL + "/readyz")
			if e == nil {
				code := resp.StatusCode
				resp.Body.Close()
				if code == 200 && nanoNow()-started > int64(200*time.Millisecond) {
					break
				}
			}
			if time.Now().After(deadline) {
				return r, fmt.Errorf("restart failed")
			}
			time.Sleep(10 * time.Millisecond)
		}
		r.RecoveryMS = float64(nanoNow()-started) / 1e6
		target.PIDs = snapshot(target.URL).PIDs
	}
	r.HostGoroutinesPeak = r.HostGoroutinesIdle
	r.Processes = len(target.PIDs)
	before, err := readGroup(target.PIDs)
	if err != nil {
		return r, err
	}
	var monitor sync.WaitGroup
	stop := make(chan struct{})
	monitor.Add(1)
	go func() {
		defer monitor.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				v, e := readGroup(target.PIDs)
				if e != nil {
					fmt.Fprintln(os.Stderr, "resource sample:", e)
					continue
				}
				r.PeakRSSBytes = max(r.PeakRSSBytes, v.RSS)
				r.PeakDescriptors = max(r.PeakDescriptors, v.Descriptors)
				vstats := snapshot(target.URL)
				r.HostGoroutinesPeak = max(r.HostGoroutinesPeak, vstats.Goroutines)
				r.ResourceSamples = append(r.ResourceSamples, vstats)
				if len(r.ResourceSamples)%50 == 0 {
					fmt.Fprintf(os.Stderr, "progress %s %s %.0fs goroutines=%d RSS=%.1fMiB descriptors=%d\n", top, o.Scenario, float64(len(r.ResourceSamples))/5, vstats.Goroutines, float64(vstats.RSS)/(1<<20), vstats.Descriptors)
				}
			}
		}
	}()
	started := nanoNow()
	r.Samples, r.Errors = load(client, target.URL, stream, workers, o.Duration)
	for _, s := range r.Samples {
		if s.WorkNS == 0 {
			r.Cancellations++
		}
	}
	r.Seconds = float64(nanoNow()-started) / 1e9
	close(stop)
	monitor.Wait()
	after, err := readGroup(target.PIDs)
	if err != nil {
		return r, err
	}
	r.CPUSeconds = after.CPUSeconds - before.CPUSeconds
	r.CPUCores = r.CPUSeconds / r.Seconds
	r.PeakRSSBytes = max(r.PeakRSSBytes, before.RSS, after.RSS)
	r.PeakDescriptors = max(r.PeakDescriptors, before.Descriptors, after.Descriptors)
	r.DescriptorKind = descriptorKind
	tr.CloseIdleConnections()
	time.Sleep(250 * time.Millisecond)
	r.HostGoroutinesAfter = hostStats(target.URL)
	r.MockFinal = snapshot(mockChild.URL)
	if (o.Scenario == "cancel" || o.Scenario == "soak") && (r.MockFinal.Active != 0 || r.MockFinal.Cancelled == 0) {
		return r, fmt.Errorf("cancellation ownership failed: %+v", r.MockFinal)
	}
	r.Requests = len(r.Samples)
	r.RPS = float64(r.Requests) / r.Seconds
	total, first, residual, firstResidual := []float64{}, []float64{}, []float64{}, []float64{}
	for _, s := range r.Samples {
		total = append(total, float64(s.TotalNS)/1e6)
		first = append(first, float64(s.FirstNS)/1e6)
		residual = append(residual, float64(s.TotalNS-s.WorkNS)/1e6)
		firstResidual = append(firstResidual, float64(s.FirstNS-s.WorkFirstNS)/1e6)
	}
	r.TotalMS = percentiles(total)
	r.FirstMS = percentiles(first)
	r.ResidualMS = percentiles(residual)
	r.FirstResidualMS = percentiles(firstResidual)
	if r.Errors > 0 {
		return r, fmt.Errorf("%s %s had %d errors", top, mode, r.Errors)
	}
	return r, nil
}

func load(client *http.Client, url string, stream bool, workers int, duration time.Duration) ([]sample, int) {
	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var samples []sample
	errors := 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cycle := 0
			var local []sample
			bad := 0
			body := []byte(fmt.Sprintf(`{"model":"mock","messages":[{"role":"user","content":%q}],"stream":%t,"validation_scenario":%q}`, strings.Repeat("x", workload.BodyBytes), stream, workload.Scenario))
			for time.Now().Before(deadline) {
				currentBody := body
				if workload.Scenario == "soak" {
					scenario := []string{"cancel", "sse", "slow"}[cycle%3]
					cycle++
					currentBody = bytes.Replace(body, []byte(`"soak"`), []byte(strconv.Quote(scenario)), 1)
				}
				s, e := request(client, url, currentBody, stream)
				if e != nil {
					bad++
					if bad == 1 {
						fmt.Fprintln(os.Stderr, "load failure", e)
					}
				} else {
					local = append(local, s)
				}
			}
			mu.Lock()
			samples = append(samples, local...)
			errors += bad
			mu.Unlock()
		}()
	}
	wg.Wait()
	return samples, errors
}
func request(client *http.Client, url string, body []byte, stream bool) (sample, error) {
	var s sample
	started := nanoNow()
	resp, err := client.Post(url+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return s, err
	}
	defer resp.Body.Close()
	firstByte := make([]byte, 1)
	_, err = io.ReadFull(resp.Body, firstByte)
	if err != nil {
		return s, err
	}
	s.FirstNS = nanoNow() - started
	if bytes.Contains(body, []byte(`"validation_scenario":"cancel"`)) {
		resp.Body.Close()
		s.TotalNS = nanoNow() - started
		return s, nil
	}
	var rest []byte
	if bytes.Contains(body, []byte(`"validation_scenario":"slow"`)) {
		var out bytes.Buffer
		buf := make([]byte, 4096)
		for {
			n, e := io.ReadFull(resp.Body, buf)
			if n > 0 {
				out.Write(buf[:n])
				time.Sleep(time.Millisecond)
			}
			if e == io.EOF || e == io.ErrUnexpectedEOF {
				break
			}
			if e != nil {
				return s, e
			}
		}
		rest = out.Bytes()
	} else {
		rest, err = io.ReadAll(resp.Body)
	}
	s.TotalNS = nanoNow() - started
	if err != nil {
		return s, err
	}
	if resp.StatusCode != 200 {
		return s, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b := append(firstByte, rest...)
	decode := func(b []byte) error {
		var p struct {
			Work  int64 `json:"bench_work_ns"`
			First int64 `json:"bench_first_ns"`
		}
		if e := json.Unmarshal(b, &p); e != nil {
			return e
		}
		if p.Work != 0 {
			s.WorkNS = p.Work
		}
		if p.First != 0 {
			s.WorkFirstNS = p.First
		}
		return nil
	}
	if !stream {
		err = decode(b)
	} else {
		scan := bufio.NewScanner(bytes.NewReader(b))
		// Slow-consumer cases intentionally produce multi-megabyte streams.
		// The scanner token is one SSE line, but its default 64 KiB ceiling is
		// an accidental harness limit and would turn a valid long stream into
		// a false "missing terminal" failure.
		scan.Buffer(make([]byte, 64<<10), 1<<20)
		terminal := false
		for scan.Scan() {
			line := scan.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			event := strings.TrimPrefix(line, "data: ")
			if event == "[DONE]" {
				terminal = true
				continue
			}
			if err = decode([]byte(event)); err != nil {
				break
			}
		}
		if err == nil {
			err = scan.Err()
		}
		if err == nil && !terminal {
			err = fmt.Errorf("missing SSE terminal")
		}
	}
	if err == nil && (s.WorkNS <= 0 || s.WorkFirstNS <= 0) {
		err = fmt.Errorf("missing mock work timings")
	}
	return s, err
}
func percentiles(v []float64) distribution {
	if len(v) == 0 {
		return distribution{}
	}
	sort.Float64s(v)
	pick := func(p float64) float64 { return v[min(len(v)-1, int(float64(len(v)-1)*p))] }
	return distribution{pick(.5), pick(.95), pick(.99)}
}

type resources struct {
	CPUSeconds       float64
	RSS, Descriptors uint64
}

func readGroup(pids []int) (resources, error) {
	var sum resources
	for _, pid := range pids {
		v, e := readProcess(pid)
		if e != nil {
			return sum, e
		}
		sum.CPUSeconds += v.CPUSeconds
		sum.RSS += v.RSS
		sum.Descriptors += v.Descriptors
	}
	return sum, nil
}
