package sdk

import (
	"context"
	"time"
)

// Kind names reserve discovery namespaces, not runnable RPC services in v1.
const (
	KindProtocol       = "protocol"
	KindUpstream       = "upstream"
	KindProvider       = "provider"
	KindAuthenticator  = "authenticator"
	KindScheduler      = "scheduler"
	KindMiddleware     = "middleware"
	KindStorage        = "storage"
	KindUsage          = "usage"
	KindBilling        = "billing"
	KindQuota          = "quota"
	KindObservability  = "observability"
	KindAdmin          = "admin"
	KindSecretProvider = "secret-provider"
)

// These small extension sketches are experimental; only the three v1 services
// have a stable wire contract. No user, account, balance or repository is defined.
type SessionAffinity interface {
	Key(context.Context, *RequestInfo) (string, bool, error)
}
type Identity struct {
	Subject string
	Claims  map[string]string
}
type Authenticator interface {
	Authenticate(context.Context, *Ingress) (*Identity, error)
}
type Middleware interface {
	Before(context.Context, *Request) error
	After(context.Context, *RequestInfo, *AttemptResult) error
}
type PutOptions struct {
	TTL      time.Duration
	IfAbsent bool
}
type Store interface {
	Get(context.Context, string) ([]byte, error)
	Put(context.Context, string, []byte, PutOptions) error
	Delete(context.Context, string) error
}
type UsageEvent struct {
	RequestID, Model                        string
	InputTokens, OutputTokens, CachedTokens int64
	Duration                                time.Duration
	Partial                                 bool
}
type UsageSink interface {
	Record(context.Context, UsageEvent) error
}
type SecretProvider interface {
	Resolve(context.Context, string) ([]byte, error)
}
