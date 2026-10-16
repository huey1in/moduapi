//go:build conformance

package conformance

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/core/gateway"
	"github.com/huey1in/moduapi/core/lifecycle"
	"github.com/huey1in/moduapi/core/router"
	pb "github.com/huey1in/moduapi/gen/plugin/v1"
	"github.com/huey1in/moduapi/runtime/manifest"
	"github.com/huey1in/moduapi/runtime/process"
	"github.com/huey1in/moduapi/runtime/registry"
	rpcclient "github.com/huey1in/moduapi/runtime/rpc"
	"github.com/huey1in/moduapi/sdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func declaration(t *testing.T, mode string) (manifest.Manifest, string) {
	t.Helper()
	m, e := manifest.Read(filepath.Join("python", "manifest.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	m.Runtime.Interpreter = os.Getenv("MODUAPI_CONFORMANCE_PYTHON")
	if m.Runtime.Interpreter == "" {
		t.Fatal("run make conformance (isolated Python environment required)")
	}
	events := filepath.Join(t.TempDir(), "events")
	m.Runtime.Args = []string{"--mode", mode, "--events", events}
	return m, events
}
func start(t *testing.T, mode string) (*process.Process, string) {
	t.Helper()
	m, events := declaration(t, mode)
	p, e := process.Start(context.Background(), m, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if t.Failed() {
			b, _ := os.ReadFile(events)
			t.Logf("events: %s", b)
		}
		p.Stop(context.Background())
		if p.ExitError() != nil {
			t.Error(p.ExitError())
		}
		if p.Conn.GetState() != connectivity.Shutdown {
			t.Error("connection not closed")
		}
	})
	return p, events
}
func resolve(p *process.Process) rpcclient.Resolve {
	return func() (*grpc.ClientConn, error) { return p.Conn, nil }
}
func until(t *testing.T, pred func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for !pred() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func has(path, event string) bool {
	b, _ := os.ReadFile(path)
	return strings.Contains(string(b), event+"\n")
}
func TestHandshakeAndConfiguration(t *testing.T) {
	list, e := manifest.Discover("python")
	if e != nil || len(list) != 1 || list[0].Runtime.Type != "interpreter" {
		t.Fatalf("interpreter discovery: %v %v", list, e)
	}
	p, _ := start(t, "normal")
	api := pb.NewLifecycleClient(p.Conn)
	if _, e := api.Handshake(context.Background(), &pb.Hello{ApiVersion: 2}); status.Code(e) != codes.FailedPrecondition {
		t.Fatal(e)
	}
	info, e := api.Handshake(context.Background(), &pb.Hello{ApiVersion: 1, Capabilities: []string{"affinity", "future"}})
	if e != nil || len(info.Capabilities) != 1 || info.Capabilities[0] != "affinity" {
		t.Fatalf("%v %v", info, e)
	}
	for _, mode := range []string{"api", "identity", "version", "capabilities"} {
		t.Run(mode, func(t *testing.T) {
			m, _ := declaration(t, mode)
			p, e := process.Start(context.Background(), m, nil)
			if e == nil {
				p.Stop(context.Background())
				t.Fatal("accepted mismatch")
			}
			if !strings.Contains(e.Error(), "mismatch") {
				t.Fatal(e)
			}
		})
	}
	for _, cfg := range []string{`{}`, `{"value":1}`, `{`, `[]`, `{"reject":true}`} {
		t.Run("config"+cfg, func(t *testing.T) {
			m, _ := declaration(t, "normal")
			p, e := process.Start(context.Background(), m, []byte(cfg))
			valid := cfg == `{}` || cfg == `{"value":1}`
			if valid != (e == nil) {
				t.Fatal(e)
			}
			if p != nil {
				p.Stop(context.Background())
			}
		})
	}
	// Invalid JSON must also be rejected by the independent Python implementation.
	if _, e := api.Configure(context.Background(), &pb.Configuration{Json: []byte(`{`)}); status.Code(e) != codes.InvalidArgument {
		t.Fatal(e)
	}
}
func TestUnknownWireFields(t *testing.T) {
	p, _ := start(t, "normal")
	original := &pb.Request{Id: "wire", Protocol: "test.echo", Model: "normal"}
	wire, _ := proto.Marshal(original)
	wire = protowire.AppendTag(wire, 19000, protowire.VarintType)
	wire = protowire.AppendVarint(wire, 7)
	req := &pb.Request{}
	if e := proto.Unmarshal(wire, req); e != nil {
		t.Fatal(e)
	}
	out, e := pb.NewProtocolClient(p.Conn).Prepare(context.Background(), &pb.Preparation{Request: req})
	if e != nil || out.GetMetadata()["unknown_seen"] != "yes" {
		t.Fatalf("Go → Python unknown field: %v %v", out, e)
	}
	unknown := out.ProtoReflect().GetUnknown()
	seen := map[protowire.Number]bool{}
	for len(unknown) > 0 {
		n, typ, k := protowire.ConsumeTag(unknown)
		if k < 0 {
			t.Fatal("bad wire")
		}
		unknown = unknown[k:]
		v := protowire.ConsumeFieldValue(n, typ, unknown)
		if v < 0 {
			t.Fatal("bad value")
		}
		seen[n] = true
		unknown = unknown[v:]
	}
	if !seen[19000] || !seen[19001] {
		t.Fatalf("Python → Go unknown fields not preserved: %v", seen)
	}
}

func TestMessageLimit(t *testing.T) {
	p, _ := start(t, "normal")
	_, e := pb.NewProtocolClient(p.Conn).Decode(context.Background(), &pb.Ingress{Body: make([]byte, sdk.MaxMessageBytes+1)})
	if status.Code(e) != codes.ResourceExhausted {
		t.Fatalf("message limit not enforced: %v", e)
	}
}

type scenario struct {
	name   string
	frames []*sdk.Frame
	valid  bool
}

func scenarios() []scenario {
	s, d, e, f := sdk.Start(200, nil), sdk.Data([]byte("x")), sdk.End(nil), sdk.Failed(&sdk.Failure{Kind: sdk.Transport, Status: 502})
	return []scenario{{"normal", []*sdk.Frame{s, d, d, e}, true}, {"empty_data", []*sdk.Frame{s, sdk.Data(nil), d, e}, true}, {"failed", []*sdk.Frame{s, d, f}, true}, {"data_first", []*sdk.Frame{d, e}, false}, {"duplicate_start", []*sdk.Frame{s, s, e}, false}, {"end_first", []*sdk.Frame{e}, false}, {"multiple_end", []*sdk.Frame{s, e, e}, false}, {"data_after_end", []*sdk.Frame{s, e, d}, false}, {"failure_after_end", []*sdk.Frame{s, e, f}, false}, {"no_terminal", []*sdk.Frame{s, d}, false}, {"oversized", []*sdk.Frame{s, sdk.Data(make([]byte, sdk.MaxChunkBytes+1)), e}, false}}
}
func TestStreamingBothDirections(t *testing.T) {
	p, _ := start(t, "normal")
	up := rpcclient.Upstream{Resolve: resolve(p)}
	for _, tc := range scenarios() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			raw, e := up.Execute(ctx, &pb.Execution{Request: &pb.Request{Model: tc.name}})
			if e != nil {
				t.Fatal(e)
			}
			defer raw.Close()
			stream := sdk.GuardStream(raw)
			for {
				_, e = stream.Recv()
				if e != nil {
					break
				}
			}
			if (e == io.EOF) != tc.valid {
				t.Fatalf("Python → Go valid=%v: %v", tc.valid, e)
			}
			transform, e := pb.NewProtocolClient(p.Conn).Transform(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if e = transform.Send(&pb.TransformInput{Value: &pb.TransformInput_Request{Request: &pb.Request{Model: "normal"}}}); e != nil {
				t.Fatal(e)
			}
			for _, f := range tc.frames {
				if transform.Send(&pb.TransformInput{Value: &pb.TransformInput_Frame{Frame: f}}) != nil {
					break
				}
			}
			_ = transform.CloseSend()
			var last *pb.Frame
			for {
				v, err := transform.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				last = v
			}
			if last == nil {
				t.Fatal("no terminal")
			}
			rejected := last.GetFailure().GetKind() == sdk.ProtocolError
			if rejected == tc.valid {
				t.Fatalf("Go → Python valid=%v last=%v", tc.valid, last)
			}
		})
	}
}
func TestErrorSemantics(t *testing.T) {
	p, _ := start(t, "normal")
	protocol := rpcclient.Protocol{Resolve: resolve(p)}
	up := rpcclient.Upstream{Resolve: resolve(p)}
	for _, kind := range []string{sdk.Local, sdk.Transport, sdk.HTTP, sdk.ProtocolError, sdk.Timeout, sdk.Downstream, sdk.PluginError} {
		t.Run(kind, func(t *testing.T) {
			_, e := protocol.Decode(context.Background(), &sdk.Ingress{Body: []byte("detail:" + kind)})
			if e == nil || sdk.FailureOf(e).Kind != kind {
				t.Fatalf("detail %v", e)
			}
			raw, e := up.Execute(context.Background(), &sdk.Execution{Request: &sdk.Request{Model: "failure:" + kind}})
			if e != nil {
				t.Fatal(e)
			}
			defer raw.Close()
			f, e := sdk.GuardStream(raw).Recv()
			if e != nil || f.GetFailure().GetKind() != kind {
				t.Fatalf("frame %v %v", f, e)
			}
		})
	}
	for code, kind := range map[string]string{"INVALID_ARGUMENT": sdk.Local, "RESOURCE_EXHAUSTED": sdk.Local, "DEADLINE_EXCEEDED": sdk.Timeout, "CANCELLED": sdk.Downstream, "INTERNAL": sdk.PluginError, "UNAVAILABLE": sdk.PluginError, "UNIMPLEMENTED": sdk.PluginError} {
		_, e := protocol.Decode(context.Background(), &sdk.Ingress{Body: []byte("grpc:" + code)})
		if e == nil || sdk.FailureOf(e).Kind != kind {
			t.Fatalf("%s: %v", code, e)
		}
	}
}
func TestHTTPStreamOwnership(t *testing.T) {
	m, events := declaration(t, "normal")
	manager := process.NewManager()
	defer manager.Close(context.Background())
	if e := manager.Start(context.Background(), []manifest.Manifest{m}, nil); e != nil {
		t.Fatal(e)
	}
	reg, e := registry.New(manager)
	if e != nil {
		t.Fatal(e)
	}
	cfg := config.Config{Routes: []router.Route{{ID: "python", Protocol: "test.echo", Model: "*", Scheduler: m.ID, Upstreams: []string{"a"}}}, Upstreams: map[string]config.Upstream{"a": {Plugin: m.ID}}}
	cfg.Defaults()
	completed := make(chan *sdk.Failure, 1)
	hooks := lifecycle.New(lifecycle.HookFunc(func(ctx context.Context, e lifecycle.Event) error {
		if e.Stage == lifecycle.Complete {
			completed <- e.Failure
		}
		return nil
	}))
	g, e := gateway.New(cfg, reg, hooks, nil)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(g)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/test", strings.NewReader("hang"))
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	buf := make([]byte, 10)
	if _, e = io.ReadFull(res.Body, buf); e != nil {
		t.Fatal(e)
	}
	cancel()
	res.Body.Close()
	select {
	case f := <-completed:
		if f.GetKind() != sdk.Downstream {
			t.Fatalf("HTTP cancellation: %v", f)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP handler did not join")
	}
	until(t, func() bool {
		return has(events, "execute_cancelled") && has(events, "execute_exited") && has(events, "transform_exited")
	}, "Python streams leaked")
	p, _ := manager.Get(m.ID)
	manager.Close(context.Background())
	if p.ExitError() != nil {
		t.Fatal(p.ExitError())
	}
	if p.Conn.GetState() != connectivity.Shutdown {
		t.Fatal("connection leaked")
	}
}
func TestForwarderJoin(t *testing.T) {
	p, events := start(t, "normal")
	before := runtime.NumGoroutine()
	for n := 0; n < 20; n++ {
		ctx, cancel := context.WithCancel(context.Background())
		up := rpcclient.Upstream{Resolve: resolve(p)}
		raw, e := up.Execute(ctx, &sdk.Execution{Request: &sdk.Request{Model: "hang"}})
		if e != nil {
			t.Fatal(e)
		}
		closed := make(chan struct{})
		var once sync.Once
		source := raw
		source.Close = func() { once.Do(func() { raw.Close(); close(closed) }) }
		transformed, e := (rpcclient.Protocol{Resolve: resolve(p)}).Transform(ctx, &sdk.Request{}, source)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = transformed.Recv(); e != nil {
			t.Fatal(e)
		}
		if _, e = transformed.Recv(); e != nil {
			t.Fatal(e)
		}
		cancel()
		done := make(chan struct{})
		go func() { transformed.Close(); close(done) }()
		until(t, func() bool {
			select {
			case <-done:
				return true
			default:
				return false
			}
		}, "forwarding goroutine was not joined")
		select {
		case <-closed:
		default:
			t.Fatal("source not closed")
		}
	}
	until(t, func() bool {
		b, _ := os.ReadFile(events)
		return strings.Count(string(b), "execute_exited\n") == 20 && strings.Count(string(b), "transform_exited\n") == 20
	}, "active Python streams remain")
	until(t, func() bool { return runtime.NumGoroutine() <= before+4 }, "goroutine growth after cancellation")
}
func TestShutdownAndEOF(t *testing.T) {
	t.Run("drain", func(t *testing.T) {
		p, events := start(t, "normal")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		stream, e := pb.NewUpstreamClient(p.Conn).Execute(ctx, &pb.Execution{Request: &pb.Request{Model: "drain"}})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = stream.Recv(); e != nil {
			t.Fatal(e)
		}
		if _, e = pb.NewLifecycleClient(p.Conn).Shutdown(ctx, &pb.Empty{}); e != nil {
			t.Fatal(e)
		}
		if _, e = pb.NewSchedulerClient(p.Conn).Select(ctx, &pb.SelectionInput{Candidates: []*pb.Candidate{{Id: "a"}}}); e == nil {
			t.Fatal("new work admitted during shutdown")
		}
		end := false
		for {
			f, e := stream.Recv()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			end = end || f.GetEnd() != nil
		}
		if !end {
			t.Fatal("existing stream not drained")
		}
		until(t, func() bool { return has(events, "process_exited") }, "process did not exit")
		select {
		case <-p.Done():
		case <-ctx.Done():
			t.Fatal("process not reaped")
		}
	})
	t.Run("stdin EOF", func(t *testing.T) {
		p, events := start(t, "normal")
		if e := p.CloseInput(); e != nil {
			t.Fatal(e)
		}
		until(t, func() bool { return has(events, "stdin_eof") }, "stdin EOF not observed")
		select {
		case <-p.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("EOF process leaked")
		}
	})
}
