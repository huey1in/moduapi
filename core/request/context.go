package request

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/huey1in/moduapi/sdk"
)

type Context struct {
	Context      context.Context
	ID           string
	StartedAt    time.Time
	Request      *sdk.Request
	Attempts     []*sdk.AttemptResult
	Committed    bool
	BytesEmitted int64
}

func New(ctx context.Context) *Context {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return &Context{Context: ctx, ID: hex.EncodeToString(b[:]), StartedAt: time.Now()}
}
