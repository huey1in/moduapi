// Package plugin hosts a plugin process. It never imports the kernel or runtime.
package plugin

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	pb "github.com/huey1in/moduapi/gen/plugin/v1"
	"github.com/huey1in/moduapi/sdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Definition struct {
	Info      *sdk.Description
	Protocol  sdk.Protocol
	Upstream  sdk.Upstream
	Scheduler sdk.Scheduler
	Configure func(context.Context, json.RawMessage) error
	Close     func() error
}
type server struct {
	pb.UnimplementedLifecycleServer
	pb.UnimplementedProtocolServer
	pb.UnimplementedUpstreamServer
	pb.UnimplementedSchedulerServer
	def      Definition
	ready    atomic.Bool
	draining atomic.Bool
	configMu sync.Mutex
	stop     chan struct{}
}

// Run reserves stdout for one bounded bootstrap record. Log only to stderr.
func Run(def Definition) error {
	diagnostic("startup", "")
	defer diagnostic("stopped", "")
	token := os.Getenv("MODUAPI_PLUGIN_TOKEN")
	if len(token) < 32 {
		return fmt.Errorf("plugin must be launched by a host")
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer lis.Close()
	s := &server{def: def, stop: make(chan struct{}, 1)}
	auth := func(ctx context.Context) error {
		md, _ := metadata.FromIncomingContext(ctx)
		got := md.Get("authorization")
		if len(got) != 1 || subtle.ConstantTimeCompare([]byte(got[0]), []byte("Bearer "+token)) != 1 {
			return status.Error(codes.Unauthenticated, "invalid host token")
		}
		return nil
	}
	g := grpc.NewServer(grpc.MaxRecvMsgSize(sdk.MaxMessageBytes), grpc.MaxSendMsgSize(sdk.MaxMessageBytes),
		grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
			if err := auth(ctx); err != nil {
				return nil, err
			}
			if !strings.Contains(info.FullMethod, ".Lifecycle/") && (!s.ready.Load() || s.draining.Load()) {
				return nil, status.Error(codes.Unavailable, "plugin is not accepting work")
			}
			return next(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, next grpc.StreamHandler) error {
			if err := auth(stream.Context()); err != nil {
				return err
			}
			if !s.ready.Load() || s.draining.Load() {
				return status.Error(codes.Unavailable, "plugin is not accepting work")
			}
			return next(srv, stream)
		}))
	pb.RegisterLifecycleServer(g, s)
	if def.Protocol != nil {
		pb.RegisterProtocolServer(g, s)
	}
	if def.Upstream != nil {
		pb.RegisterUpstreamServer(g, s)
	}
	if def.Scheduler != nil {
		pb.RegisterSchedulerServer(g, s)
	}
	if def.Close != nil {
		defer def.Close()
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"api_version": sdk.APIVersion, "address": lis.Addr().String()}); err != nil {
		return err
	}
	// Parent death closes stdin. This also bounds orphan lifetime on host failure.
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); g.Stop() }()
	go func() { <-s.stop; g.GracefulStop() }()
	err = g.Serve(lis)
	if err == grpc.ErrServerStopped {
		return nil
	}
	return err
}
func (s *server) Handshake(ctx context.Context, h *pb.Hello) (*pb.Description, error) {
	if h.ApiVersion != sdk.APIVersion {
		return nil, status.Error(codes.FailedPrecondition, "unsupported plugin API")
	}
	r := &pb.Description{Id: s.def.Info.Id, Version: s.def.Info.Version, ApiVersion: sdk.APIVersion, Provides: s.def.Info.Provides, Endpoints: s.def.Info.Endpoints}
	for _, want := range s.def.Info.Capabilities {
		for _, have := range h.Capabilities {
			if want == have {
				r.Capabilities = append(r.Capabilities, want)
				break
			}
		}
	}
	return r, nil
}
func (s *server) Configure(ctx context.Context, c *pb.Configuration) (*pb.Empty, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if s.draining.Load() {
		return nil, status.Error(codes.Unavailable, "plugin is draining")
	}
	if s.ready.Load() {
		return nil, status.Error(codes.FailedPrecondition, "already configured")
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(c.Json, &obj) != nil || obj == nil {
		return nil, status.Error(codes.InvalidArgument, "configuration must be an object")
	}
	if s.def.Configure == nil && len(obj) != 0 {
		return nil, status.Error(codes.InvalidArgument, "this plugin accepts only empty process configuration")
	}
	if s.def.Configure != nil {
		if err := s.def.Configure(ctx, c.Json); err != nil {
			return nil, status.Error(codes.InvalidArgument, "plugin configuration rejected")
		}
	}
	s.ready.Store(true)
	diagnostic("ready", "")
	return &pb.Empty{}, nil
}
func (s *server) Health(context.Context, *pb.Empty) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{Ready: s.ready.Load() && !s.draining.Load()}, nil
}
func (s *server) Shutdown(context.Context, *pb.Empty) (*pb.Empty, error) {
	s.draining.Store(true)
	diagnostic("draining", "")
	s.ready.Store(false)
	select {
	case s.stop <- struct{}{}:
	default:
	}
	return &pb.Empty{}, nil
}

func diagnostic(event, code string) {
	_ = json.NewEncoder(os.Stderr).Encode(struct {
		Event string `json:"event"`
		Code  string `json:"code,omitempty"`
	}{event, code})
}
func wireError(err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := sdk.FailureOf(err); ctxErr.Kind == sdk.Downstream {
		return status.Error(codes.Canceled, "cancelled")
	}
	f := sdk.FailureOf(err)
	st := status.New(codes.InvalidArgument, f.Message)
	with, e := st.WithDetails(f)
	if e != nil {
		return st.Err()
	}
	return with.Err()
}
func (s *server) Decode(ctx context.Context, r *pb.Ingress) (*pb.Request, error) {
	v, e := s.def.Protocol.Decode(ctx, r)
	return v, wireError(e)
}
func (s *server) Prepare(ctx context.Context, r *pb.Preparation) (*pb.Request, error) {
	v, e := s.def.Protocol.Prepare(ctx, r)
	return v, wireError(e)
}
func (s *server) Transform(stream grpc.BidiStreamingServer[pb.TransformInput, pb.Frame]) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	r := first.GetRequest()
	if r == nil {
		return status.Error(codes.InvalidArgument, "request frame required")
	}
	recv := func() (*sdk.Frame, error) {
		v, e := stream.Recv()
		if e != nil {
			return nil, e
		}
		if v.GetFrame() == nil {
			return nil, status.Error(codes.InvalidArgument, "response frame required")
		}
		return v.GetFrame(), nil
	}
	guard := sdk.GuardStream(sdk.Stream{Recv: recv, Close: func() {}})
	if err := s.def.Protocol.Transform(stream.Context(), r, guard.Recv, stream.Send); err != nil {
		return stream.Send(sdk.Failed(sdk.FailureOf(err)))
	}
	return nil
}
func (s *server) Execute(r *pb.Execution, stream grpc.ServerStreamingServer[pb.Frame]) error {
	if err := s.def.Upstream.Execute(stream.Context(), r, stream.Send); err != nil {
		return stream.Send(sdk.Failed(sdk.FailureOf(err)))
	}
	return nil
}
func (s *server) Select(ctx context.Context, r *pb.SelectionInput) (*pb.Selection, error) {
	v, e := s.def.Scheduler.Select(ctx, r)
	return v, wireError(e)
}
func (s *server) Report(ctx context.Context, r *pb.AttemptResult) (*pb.Empty, error) {
	return &pb.Empty{}, wireError(s.def.Scheduler.Report(ctx, r))
}
