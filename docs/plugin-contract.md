# ModuAPI process contract v1

Positioning scope: these are three typed component services, not a promise to grow
a business PluginKind platform. One package/process may provide multiple services;
service separation does not require one process per role. Plugins are trusted
operator-installed code, not untrusted downloads. [ADR 0009](adr/0009-moduapi-positioning.md)
freezes kind expansion while preserving the v1 semantics below.

Status: tested v1 baseline, 2026-10-16. The normative wire definition is
[`plugin.proto`](../proto/moduapi/plugin/v1/plugin.proto). Go helpers are optional.
The independent [Python fixture](../tests/conformance/python/plugin.py) imports
only generated protobuf types and Python gRPC libraries.

## Compatibility and negotiation

`api_version: 1` identifies package `moduapi.plugin.v1`. Incompatible semantics
require a new versioned package. Add fields/methods without renumbering existing
fields; reserve retired numbers. Preserve unknown protobuf fields on pass-through
messages. Unknown fields are not errors; an unknown Frame oneof alternative alone
cannot be interpreted as a valid frame. JSON conversion is not wire pass-through.
Both directions are exercised with manually appended wire fields 50000/50001.
See [protobuf guidance](https://protobuf.dev/programming-guides/proto3/#unknowns).

Host sends supported capabilities in Hello. Plugin returns the intersection with
its declared capabilities. All manifest capabilities are required for this package:
host rejects a missing negotiated capability. Unknown host capabilities are ignored.
ID, strict SemVer, API version and provided service kinds must match the manifest.
Endpoints are immutable across restart. v1 currently runs Protocol, Upstream and
Scheduler; other kind names reserve future namespaces, not implemented services.
No ValidateConfiguration RPC was added: optional/UNIMPLEMENTED negotiation is deferred.

## Bootstrap and runtime

Native `runtime.type: process` resolves command **inside its package**, including
symlink resolution. `./plugin` remains supported. `type: interpreter` resolves
`interpreter` as a bare PATH name or absolute path; command is a package-local
script, never a shell expression. Args are literal argv entries. The host sets cwd
to the package and passes only OS essentials and explicitly granted env secrets.
Interpreter selection is trusted operator configuration; pin a virtualenv absolute
path for reproducibility. Package containment does not authenticate the interpreter.

Host provides a random `MODUAPI_PLUGIN_TOKEN`. Every RPC requires metadata
`authorization: Bearer <token>`. Child listens on IPv4 loopback with an ephemeral
port and writes exactly one newline-terminated JSON bootstrap record to stdout:
`{"api_version":1,"address":"127.0.0.1:port"}` (maximum 4KiB). Stdout is not a log
channel. Startup budget is five seconds including handshake/configure/health.
Process isolation != security sandbox. Grants are a contract, not OS confinement.

## Lifecycle and admission

Discover → validate → spawn → bootstrap → Handshake → Configure → Health.ready →
serve. Configure receives UTF-8 JSON **object bytes**, including `{}`; invalid JSON,
non-objects and rejected config fail startup and are reaped, without infinite retry.
Configure succeeds once. Serialize configuration; work is rejected before ready or
after draining. Health is not proof that a particular candidate is healthy.

Shutdown marks admission closed before acknowledging. Already admitted calls drain,
then the server and process exit. Host allows two seconds then kills and reaps.
Close resources and join goroutines before process exit. Parent stdin EOF requests
immediate stop/cancellation, not an unbounded drain. An intentional stop is not a
runtime crash. See [gRPC graceful shutdown](https://grpc.io/docs/guides/server-graceful-stop/).

## Streams and ownership

Execute is server streaming. Transform is bidirectional: first input is exactly one
Request; subsequent inputs are Frames. Each response stream is:

* `start, data*, end`, or `start, data*, failure`;
* a single early `failure` is allowed for pre-header rejection.

Start status is 200..599 and appears once. End requires start. Data frames are
0..32768 bytes. Exactly one terminal is followed **immediately by clean gRPC EOF**.
No data/start/terminal after terminal. Bare EOF is a protocol failure. The guard
holds only the terminal while reading EOF; it never buffers the full response.
The caller's first-byte/idle/total timeout bounds a plugin that withholds EOF.
A gRPC error after a terminal invalidates completion. Protocol completion such as
SSE `[DONE]` may finish Transform early: host then cancels the raw upstream leg.
Unconsumed upstream bytes after that cancellation cannot be validated and are not
reported as a successfully completed raw stream.

The creator owns each stream until Close, even on early completion, failure or
cancellation. Transform owns its forwarding goroutine and source stream. Close
cancels both legs, closes source, and joins the sender. Consumers apply synchronous
backpressure; no unbounded queue. All RPCs inherit the HTTP/attempt context.
Disconnect cancels HTTP → gRPC → plugin → upstream. Plugins must observe cancellation
and release work. Cleanup Report alone uses a separate bounded context.
Once downstream output is committed, failover/replay is prohibited by the kernel.

Both gRPC send/receive messages are limited to 10MiB, independently of the 32KiB
data-frame limit and the kernel's ingress cap (4MiB default, 8MiB maximum).

## Errors

Failure.kind is one of local_rejection, upstream_transport, upstream_http,
upstream_protocol, timeout, downstream_cancellation, plugin_error. Failure is
returned as a terminal Frame or a `google.rpc.Status` Any detail with type URL
`type.googleapis.com/moduapi.plugin.v1.Failure`. Preserve its kind/status/code.
Messages must be safe for clients: no headers, credentials, body, raw affinity or
credential-bearing URLs. RequestSent is conservative, not an idempotency guarantee.

Without a Failure detail, Canceled maps to downstream cancellation; DeadlineExceeded
to timeout; InvalidArgument/FailedPrecondition/OutOfRange/PermissionDenied/
Unauthenticated/ResourceExhausted/NotFound/AlreadyExists to local rejection;
Internal/Unknown/DataLoss to plugin_internal; Unimplemented to plugin_unimplemented;
remaining transport availability errors to plugin_unavailable (both plugin_error).
An RPC UNAVAILABLE does **not** prove an upstream transport failure. Explicit
Failure is required to attribute that fault to an upstream. Never echo raw gRPC
status text to clients. See [gRPC status codes](https://grpc.io/docs/guides/status-codes/).

## Scheduling and leases

Router supplies full candidates, exclusions and attempt number. Scheduler owns all
policy/state. Candidate metadata and RequestInfo metadata are opaque to the kernel.
Select may return an opaque lease; Report echoes it on every attempt outcome.
Report delivery is best effort, so leases must expire and release idempotently.
Cancellation before Select returns must release locally; the unavoidable loss of a
response after return requires expiry. No exactly-once RPC or durable reservation
guarantee exists. Scheduler process restart discards its local state.

Run `make conformance` (or `go run ./tools/conformance` on any platform) to create a
private virtualenv, install pinned dependencies, regenerate Python bindings and run
the race-enabled Go↔Python suite. Ordinary `go test ./...` never requires Python.

## Independent protocol validation

The reference [Anthropic Messages process](../plugins/reference/protocol-anthropic/README.md)
implements this contract without the Go SDK. It demonstrates that a protocol
author can choose another language, preserve opaque request bytes, interpret a
different JSON/SSE vocabulary, and still use the existing generic HTTP upstream
and scheduler. Its manifest uses `runtime.type: interpreter`; the host resolves
the interpreter from PATH or an explicitly configured absolute path while the
script remains package-local. This is a reference subset, not a complete
provider implementation.

`make conformance` includes both the original Python lifecycle fixture and the
Anthropic process/gateway test. The latter starts a real plugin process and a
real mock Messages endpoint, then exercises non-stream and SSE requests. No
Core code or protobuf revision is required to add the endpoint.

## Experimental embedding surface

The [`embedding`](../embedding/embedding.go) package is intentionally small:

```go
rt, err := embedding.Start(ctx, config)
defer rt.Close(shutdownCtx)
http.HandleFunc("/", rt.ServeHTTP)
```

The host owns the HTTP listener, TLS, HTTP server drain, and admission policy.
`Start` starts the configured process bundles; `ServeHTTP` binds request
cancellation to the runtime; `Close` rejects new requests, cancels admitted
ones, stops children and waits for reaping. The package is an experimental
candidate API. Its exported names may change after the Phase 3A integration
evidence; `internal/app`, registry implementation details and official plugin
packages remain internal implementation choices.

## Placement

Protocol, Scheduler and Upstream are typed services, not a process-count
prescription. A single package may provide all three services. Phase 3A records
the split/bundle/in-process trade-off and keeps the v1 stream and cancellation
semantics identical across placements. Process isolation is still trusted
operator isolation, not an untrusted-code sandbox.
