// Package sdk is the protocol-neutral Go view of the v1 plugin contract.
// Wire types are aliases: opaque payloads do not need a second JSON encoding.
package sdk

import (
	"context"
	"errors"
	"net/http"

	pb "github.com/huey1in/moduapi/gen/plugin/v1"
)

const APIVersion = 1
const GatewayVersion = "0.1.0"
const MaxMessageBytes = 10 << 20
const MaxChunkBytes = 32 << 10

var HostCapabilities = []string{"streaming", "usage", "affinity"}

type Request = pb.Request
type Ingress = pb.Ingress
type Preparation = pb.Preparation
type Execution = pb.Execution
type Frame = pb.Frame
type Failure = pb.Failure
type Usage = pb.Usage
type Candidate = pb.Candidate
type SelectionInput = pb.SelectionInput
type Selection = pb.Selection
type AttemptResult = pb.AttemptResult
type RequestInfo = pb.RequestInfo
type Endpoint = pb.Endpoint
type Description = pb.Description
type Receive func() (*Frame, error)
type Send func(*Frame) error

const (
	Local         = "local_rejection"
	Transport     = "upstream_transport"
	HTTP          = "upstream_http"
	ProtocolError = "upstream_protocol"
	Downstream    = "downstream_cancellation"
	Timeout       = "timeout"
	PluginError   = "plugin_error"
)

type Error struct{ Failure *Failure }

func (e *Error) Error() string { return e.Failure.Message }
func Fail(kind, code, message string, status int) error {
	return &Error{&Failure{Kind: kind, Code: code, Message: message, Status: int32(status)}}
}
func FailureOf(err error) *Failure {
	var e *Error
	if errors.As(err, &e) {
		return e.Failure
	}
	if errors.Is(err, context.Canceled) {
		return &Failure{Kind: Downstream, Code: "cancelled", Message: "request cancelled", Status: 499}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Failure{Kind: Timeout, Code: "deadline", Message: "request deadline exceeded", Status: 504}
	}
	return &Failure{Kind: PluginError, Code: "plugin_unavailable", Message: "plugin call failed", Status: 502}
}
func Start(status int, headers http.Header) *Frame {
	return &Frame{Value: &pb.Frame_Start{Start: &pb.ResponseStart{Status: int32(status), Headers: Headers(headers)}}}
}
func Data(b []byte) *Frame     { return &Frame{Value: &pb.Frame_Data{Data: b}} }
func End(u *Usage) *Frame      { return &Frame{Value: &pb.Frame_End{End: &pb.ResponseEnd{Usage: u}}} }
func Failed(f *Failure) *Frame { return &Frame{Value: &pb.Frame_Failure{Failure: f}} }
func Headers(h http.Header) map[string]*pb.Values {
	r := make(map[string]*pb.Values, len(h))
	for k, v := range h {
		r[k] = &pb.Values{Values: append([]string(nil), v...)}
	}
	return r
}
func HTTPHeaders(h map[string]*pb.Values) http.Header {
	r := make(http.Header, len(h))
	for k, v := range h {
		if v != nil {
			for _, value := range v.Values {
				r.Add(k, value)
			}
		}
	}
	return r
}
func Info(r *Request) *RequestInfo {
	return &RequestInfo{Id: r.Id, Protocol: r.Protocol, Model: r.Model, Metadata: r.Metadata}
}

type Protocol interface {
	Decode(context.Context, *Ingress) (*Request, error)
	Prepare(context.Context, *Preparation) (*Request, error)
	Transform(context.Context, *Request, Receive, Send) error
}
type Upstream interface {
	Execute(context.Context, *Execution, Send) error
}
type Scheduler interface {
	Select(context.Context, *SelectionInput) (*Selection, error)
	Report(context.Context, *AttemptResult) error
}

// Client SPI adds ownership to the streaming calls: every receive has a Close.
type Stream struct {
	Recv  Receive
	Close func()
}
type ProtocolClient interface {
	Decode(context.Context, *Ingress) (*Request, error)
	Prepare(context.Context, *Preparation) (*Request, error)
	Transform(context.Context, *Request, Stream) (Stream, error)
}
type UpstreamClient interface {
	Execute(context.Context, *Execution) (Stream, error)
}
type Registry interface {
	Endpoint(method, path string) (string, string, bool)
	Protocol(id string) (ProtocolClient, error)
	Upstream(id string) (UpstreamClient, error)
	Scheduler(id string) (Scheduler, error)
	Ready() bool
}
