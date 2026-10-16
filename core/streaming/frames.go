// Package streaming validates protocol-neutral response framing and ownership.
package streaming

import (
	"github.com/huey1in/moduapi/sdk"
)

// Guard does not prefetch data; it verifies RPC EOF before delivering a terminal.
func Guard(source sdk.Stream) sdk.Stream { return sdk.GuardStream(source) }
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
