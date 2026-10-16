package streaming

import (
	"github.com/huey1in/moduapi/sdk"
	"io"
	"testing"
)

func sequence(frames ...*sdk.Frame) sdk.Stream {
	i := 0
	return sdk.Stream{Close: func() {}, Recv: func() (*sdk.Frame, error) {
		if i == len(frames) {
			return nil, io.EOF
		}
		v := frames[i]
		i++
		return v, nil
	}}
}
func TestFrameContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames []*sdk.Frame
		valid  bool
	}{
		{"normal", []*sdk.Frame{sdk.Start(200, nil), sdk.Data([]byte("x")), sdk.End(nil)}, true},
		{"empty data", []*sdk.Frame{sdk.Start(200, nil), sdk.Data(nil), sdk.End(nil)}, true},
		{"early failure", []*sdk.Frame{sdk.Failed(&sdk.Failure{Kind: sdk.Transport})}, true},
		{"end first", []*sdk.Frame{sdk.End(nil)}, false},
		{"multiple end", []*sdk.Frame{sdk.Start(200, nil), sdk.End(nil), sdk.End(nil)}, false},
		{"data after end", []*sdk.Frame{sdk.Start(200, nil), sdk.End(nil), sdk.Data([]byte("x"))}, false},
		{"failure after end", []*sdk.Frame{sdk.Start(200, nil), sdk.End(nil), sdk.Failed(&sdk.Failure{Kind: sdk.Transport})}, false},
		{"data first", []*sdk.Frame{sdk.Data([]byte("x"))}, false},
		{"duplicate start", []*sdk.Frame{sdk.Start(200, nil), sdk.Start(200, nil)}, false},
		{"missing terminal", []*sdk.Frame{sdk.Start(200, nil), sdk.Data([]byte("x"))}, false},
		{"oversized frame", []*sdk.Frame{sdk.Start(200, nil), sdk.Data(make([]byte, sdk.MaxChunkBytes+1))}, false},
		{"invalid status", []*sdk.Frame{sdk.Start(99, nil)}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			s := Guard(sequence(tc.frames...))
			var err error
			for {
				_, err = s.Recv()
				if err != nil {
					break
				}
			}
			if (err == io.EOF) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
