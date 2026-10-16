// Package placement contains audit controls, not supported execution backends.
// It deliberately imports reference implementations; the kernel never does.
package placement

import (
	"context"
	"fmt"
	"io"
	"sync"

	openai "github.com/huey1in/moduapi/plugins/official/protocol-openai"
	single "github.com/huey1in/moduapi/plugins/official/scheduler-single"
	httpupstream "github.com/huey1in/moduapi/plugins/official/upstream-http"
	"github.com/huey1in/moduapi/sdk"
	"github.com/huey1in/moduapi/sdk/plugin"
	"google.golang.org/protobuf/proto"
)

func Definition() plugin.Definition {
	u := httpupstream.New([]string{"127.0.0.1"})
	return plugin.Definition{Info: &sdk.Description{Id: "validation.bundle", Version: "0.1.0", Provides: []string{"protocol", "scheduler", "upstream"}, Capabilities: []string{"streaming", "usage", "affinity"}, Endpoints: []*sdk.Endpoint{{Method: "POST", Path: "/v1/chat/completions", Protocol: "openai.chat"}}}, Protocol: openai.Protocol{}, Scheduler: single.Scheduler{}, Upstream: u, Close: u.Close}
}

type Local struct{ def plugin.Definition }

func New() *Local             { return &Local{Definition()} }
func (r *Local) Close() error { return r.def.Close() }
func (*Local) Endpoint(method, path string) (string, string, bool) {
	return "local", "openai.chat", method == "POST" && path == "/v1/chat/completions"
}
func (*Local) Ready() bool                                      { return true }
func (r *Local) Protocol(id string) (sdk.ProtocolClient, error) { return protocol{r.def.Protocol}, nil }
func (r *Local) Upstream(id string) (sdk.UpstreamClient, error) { return upstream{r.def.Upstream}, nil }
func (r *Local) Scheduler(id string) (sdk.Scheduler, error)     { return r.def.Scheduler, nil }

// A zero-buffer channel preserves backpressure. Close cancels and joins the
// producer before returning, including when a protocol finishes early.
func stream(ctx context.Context, produce func(context.Context, sdk.Send) error, source *sdk.Stream) sdk.Stream {
	ctx, cancel := context.WithCancel(ctx)
	frames := make(chan *sdk.Frame)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(frames)
		if source != nil {
			defer source.Close()
		}
		send := func(f *sdk.Frame) error {
			// gRPC normally copies a frame before the plugin returns. The local
			// ablation must preserve the same ownership contract; upstream HTTP
			// reuses its read buffer after Send returns.
			f = proto.Clone(f).(*sdk.Frame)
			select {
			case frames <- f:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err := produce(ctx, send); err != nil {
			_ = send(sdk.Failed(sdk.FailureOf(err)))
		}
	}()
	var once sync.Once
	return sdk.Stream{Recv: func() (*sdk.Frame, error) {
		select {
		case f, ok := <-frames:
			if !ok {
				return nil, io.EOF
			}
			return f, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, Close: func() {
		once.Do(func() {
			cancel()
			if source != nil {
				source.Close()
			}
			<-done
		})
	}}
}

type protocol struct{ sdk.Protocol }

func (p protocol) Transform(ctx context.Context, r *sdk.Request, src sdk.Stream) (sdk.Stream, error) {
	src = sdk.GuardStream(src)
	return stream(ctx, func(c context.Context, send sdk.Send) error { return p.Protocol.Transform(c, r, src.Recv, send) }, &src), nil
}

type upstream struct{ sdk.Upstream }

func (u upstream) Execute(ctx context.Context, r *sdk.Execution) (sdk.Stream, error) {
	return stream(ctx, func(c context.Context, s sdk.Send) error { return u.Upstream.Execute(c, r, s) }, nil), nil
}

// ValidateIDs guards the explicit control assembly, not arbitrary user plugins.
func ValidateIDs(scheduler, upstream string) error {
	if scheduler != "local" || upstream != "local" {
		return fmt.Errorf("invalid local control binding")
	}
	return nil
}
