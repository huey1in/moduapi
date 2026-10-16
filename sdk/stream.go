package sdk

import (
	"io"
)

// GuardStream owns no goroutine. A terminal is delivered only after clean RPC
// EOF: this catches trailing frames without buffering response data. The caller
// must supply a deadline and Close on every path, including early completion.
func GuardStream(source Stream) Stream {
	started, terminal := false, false
	return Stream{Close: source.Close, Recv: func() (*Frame, error) {
		if terminal {
			return nil, io.EOF
		}
		bad := func(code string) (*Frame, error) {
			return nil, Fail(ProtocolError, code, "plugin emitted an invalid response sequence", 502)
		}
		f, err := source.Recv()
		if err == io.EOF {
			return bad("missing_terminal")
		}
		if err != nil {
			return nil, err
		}
		if f == nil {
			return bad("invalid_frame")
		}
		switch {
		case f.GetStart() != nil:
			if started || f.GetStart().Status < 200 || f.GetStart().Status > 599 {
				return bad("invalid_frame")
			}
			started = true
		case f.GetEnd() != nil:
			if !started {
				return bad("invalid_frame")
			}
			terminal = true
		case f.GetFailure() != nil:
			terminal = true
		default:
			// Test the oneof, since an empty data payload is legal.
			if !IsData(f) || !started || len(f.GetData()) > MaxChunkBytes {
				return bad("invalid_frame")
			}
		}
		if terminal {
			_, err := source.Recv()
			if err != io.EOF {
				if err != nil {
					return nil, err
				}
				return bad("frame_after_terminal")
			}
		}
		return f, nil
	}}
}
