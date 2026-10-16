// Package streaming validates protocol-neutral response framing and ownership.
package streaming

import (
	"io"

	"github.com/huey1in/moduapi/sdk"
)

// Guard does not prefetch: backpressure remains on the consumer's call stack.
func Guard(source sdk.Stream) sdk.Stream {
	started, terminal := false, false
	return sdk.Stream{Close: source.Close, Recv: func() (*sdk.Frame, error) {
		if terminal {
			return nil, io.EOF
		}
		f, e := source.Recv()
		if e != nil {
			if e == io.EOF {
				return nil, sdk.Fail(sdk.ProtocolError, "missing_terminal", "plugin stream ended without a terminal frame", 502)
			}
			return nil, e
		}
		bad := func() (*sdk.Frame, error) {
			return nil, sdk.Fail(sdk.ProtocolError, "invalid_frame", "plugin emitted an invalid response sequence", 502)
		}
		if f == nil {
			return bad()
		}
		switch {
		case f.GetStart() != nil:
			if started || f.GetStart().Status < 200 || f.GetStart().Status > 599 {
				return bad()
			}
			started = true
		case f.GetData() != nil:
			if !started || len(f.GetData()) > sdk.MaxChunkBytes {
				return bad()
			}
		case f.GetEnd() != nil:
			if !started {
				return bad()
			}
			terminal = true
		case f.GetFailure() != nil:
			terminal = true
		default:
			return bad()
		}
		return f, nil
	}}
}
func Prepend(first *sdk.Frame, s sdk.Stream) sdk.Stream {
	pending := true
	return sdk.Stream{Close: s.Close, Recv: func() (*sdk.Frame, error) {
		if pending {
			pending = false
			return first, nil
		}
		return s.Recv()
	}}
}
