# Embedding API: retain experimentally, narrow only against a real host

Reviewed 2026-10-17 at `dfde6e6`. Recommendation: **retain the experimental
surface for validation; do not stabilize or expand it**. No API change is made
by this review. Delete it if the embedding cohort fails the demand gate and no
supported use case remains; exported symbols alone do not justify maintenance.

Evidence: [`embedding.go`](../../embedding/embedding.go),
[`config.Config`](../../core/config/config.go),
[`router.Route`](../../core/router/router.go),
[`app.Start`](../../internal/app/app.go),
[`Registry.Ready`](../../runtime/registry/registry.go).

| Operation | Actual behavior | External-user burden / recommendation |
|---|---|---|
| `Start(ctx, config.Config)` | Defaults/validates wiring, discovers **all** packages in a directory, starts/negotiates them and builds the gateway. Startup context is not the runtime lifetime. | Retain for experiments. Caller must own bundle selection, opaque plugin configuration and an explicit `Close`; cancelling the startup context later does not close the runtime. Public signature couples to `core/config`; programmatic routes also import `core/router` and its retry shape. All discovered bundles are required even if unused. |
| `Ready() bool` | Returns local runtime/registry readiness at that instant; process health polling updates readiness separately. | Retain as a readiness probe, not a wait, guarantee for the next call, or detailed health API. A false result has no cause/affected-component information. Do not invent a diagnostics API before a host requests it. |
| `ServeHTTP` | Standard `http.Handler`; attaches cancellation from the runtime and incoming request; rejects new calls after close. Kernel owns route/retry/stream commitment. | Retain: this is the useful embedding seam. Host owns listener/TLS, authentication/admission outside the Kernel, routing mounts, timeouts and HTTP drain. Mounting under a prefix may need `StripPrefix`; plugin endpoint paths remain exact. The Handler also exposes Kernel health paths. |
| `Close(ctx)` | Once-only stop: rejects admission, cancels active calls, stops/reaps children, waits for active handlers. No error return. It can wait for reaping beyond the supplied context. | Retain with the current explicit caveat. It is **not** a hard shutdown deadline or a normal request drain. Call the host HTTP server's `Shutdown` first for graceful completion. Only a participant's demonstrated need should drive a changed result/deadline contract. |

## Four methods are not the whole public contract

The real dependency surface also contains `config.Config`, `Plugins`, `Upstream`,
`Object`, `Server`/`Duration` and `router.Route`/`Retry`, plus YAML path/default
semantics if `config.Load` is used. These are exported and importable; they are
not evidence of a stable consumer boundary. The host must treat configuration
maps/slices as immutable after startup, not assume deep defensive copying.
The package internally uses `internal/app`, so the consumer cannot substitute
the composition root or inject hooks through `embedding.Start` today. Loading a
YAML file avoids a direct `core/router` import but retains the schema dependency.

The existing `package embedding_test` test is 52 LOC and accesses internal
build/mock helpers as fixtures. It directly invokes the Handler with an
`httptest.ResponseRecorder`; it does **not** create a downstream HTTP listener.
It is useful boundary coverage, but is not by itself an independent module or
external adopter. The [host validation brief](embedding-host-validation.md)
adds a documented separate-module reproduction with standard-library mock and
real HTTP listeners, without importing any `internal` package. This still is
our own exercise, not an adopter's evidence.

## Stable candidates and implementation details

The Handler seam and explicit ownership of startup/shutdown are stable
**candidates**. Typed wire services retain their existing versioning policy.
Directory discovery, registry/manager layout, `internal/app`, test/build helpers
and benchmark placement adapters are implementation details. The lower-level
`core/*` and `runtime/*` exported types should not gain stability promises just
because the experimental wrapper currently uses some of them.

If an actual host qualifies, prefer a **narrow consumer-owned configuration
boundary** over exposing process managers/registries. Candidate changes could
include an embedding configuration projection and explicit installed-bundle
selection, reusing existing schema semantics. A type alias alone would not
decouple the contract. First record which current fields the host uses and
what upgrade breaks; then choose the smallest additive migration. Do not build
a Builder, universal pipeline, callback bus or Host Capability API speculatively.

The lack of an owner-selected project LICENSE is also a distribution/adoption
blocker; this review does not choose one or imply permission to redistribute.
No recommendation here is a legal determination.
