// Package rpc adapts wire clients to the SDK ownership and error contracts.
package rpc

import (
	"context"
	"io"
	"sync"

	pb "github.com/huey1in/moduapi/gen/plugin/v1"
	"github.com/huey1in/moduapi/sdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Resolve func() (*grpc.ClientConn, error)

func decodeError(err error) error {
	if err == nil {
		return nil
	}
	s := status.Convert(err)
	for _, d := range s.Details() {
		if f, ok := d.(*pb.Failure); ok {
			return &sdk.Error{Failure: f}
		}
	}
	switch s.Code() {
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	}
	return sdk.Fail(sdk.PluginError, "plugin_unavailable", "plugin call failed", 502)
}

type Protocol struct{ Resolve Resolve }

func (p Protocol) Decode(ctx context.Context, r *sdk.Ingress) (*sdk.Request, error) {
	c, e := p.Resolve()
	if e != nil {
		return nil, e
	}
	v, e := pb.NewProtocolClient(c).Decode(ctx, r)
	return v, decodeError(e)
}
func (p Protocol) Prepare(ctx context.Context, r *sdk.Preparation) (*sdk.Request, error) {
	c, e := p.Resolve()
	if e != nil {
		return nil, e
	}
	v, e := pb.NewProtocolClient(c).Prepare(ctx, r)
	return v, decodeError(e)
}
func (p Protocol) Transform(ctx context.Context, r *sdk.Request, source sdk.Stream) (sdk.Stream, error) {
	c, e := p.Resolve()
	if e != nil {
		source.Close()
		return sdk.Stream{}, e
	}
	ctx, cancel := context.WithCancel(ctx)
	s, e := pb.NewProtocolClient(c).Transform(ctx)
	if e != nil {
		cancel()
		source.Close()
		return sdk.Stream{}, decodeError(e)
	}
	if e = s.Send(&pb.TransformInput{Value: &pb.TransformInput_Request{Request: r}}); e != nil {
		cancel()
		source.Close()
		return sdk.Stream{}, decodeError(e)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer s.CloseSend()
		defer source.Close()
		for {
			f, e := source.Recv()
			if e == io.EOF {
				return
			}
			if e != nil {
				f = sdk.Failed(sdk.FailureOf(e))
			}
			if s.Send(&pb.TransformInput{Value: &pb.TransformInput_Frame{Frame: f}}) != nil {
				return
			}
			if e != nil || f.GetEnd() != nil || f.GetFailure() != nil {
				return
			}
		}
	}()
	var once sync.Once
	return sdk.Stream{Recv: func() (*sdk.Frame, error) { v, e := s.Recv(); return v, decodeEOF(e) }, Close: func() { once.Do(func() { cancel(); source.Close(); <-done }) }}, nil
}
func decodeEOF(e error) error {
	if e == io.EOF {
		return e
	}
	return decodeError(e)
}

type Upstream struct{ Resolve Resolve }

func (p Upstream) Execute(ctx context.Context, r *sdk.Execution) (sdk.Stream, error) {
	c, e := p.Resolve()
	if e != nil {
		return sdk.Stream{}, e
	}
	ctx, cancel := context.WithCancel(ctx)
	s, e := pb.NewUpstreamClient(c).Execute(ctx, r)
	if e != nil {
		cancel()
		return sdk.Stream{}, decodeError(e)
	}
	return sdk.Stream{Recv: func() (*sdk.Frame, error) { v, e := s.Recv(); return v, decodeEOF(e) }, Close: cancel}, nil
}

type Scheduler struct{ Resolve Resolve }

func (p Scheduler) Select(ctx context.Context, r *sdk.SelectionInput) (*sdk.Selection, error) {
	c, e := p.Resolve()
	if e != nil {
		return nil, e
	}
	v, e := pb.NewSchedulerClient(c).Select(ctx, r)
	return v, decodeError(e)
}
func (p Scheduler) Report(ctx context.Context, r *sdk.AttemptResult) error {
	c, e := p.Resolve()
	if e != nil {
		return e
	}
	_, e = pb.NewSchedulerClient(c).Report(ctx, r)
	return decodeError(e)
}
