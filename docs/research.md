# Source research before implementation

The subsequent [competitive architecture audit](research/competitive-architecture-audit.md)
extends this initial two-project study to APISIX, Envoy ext_proc, Bifrost, llm-d
EPP, llmgate and LiteLLM, including current New API Task Plugin and Sub2API HostService
contracts. Its pinned inventory and thirty-dimension matrix supersede any inference
that these basic plugin/scheduling mechanisms are unique. See ADR 0009 before
continuing the old feature roadmap.

Read on 2026-10-16, pinned to:

- [New API d032204](https://github.com/QuantumNous/new-api/tree/d032204733db31208f10bac118b6e683aab3bf29), root LICENSE: AGPL-3.0.
- [Sub2API b0a02a3](https://github.com/Wei-Shaw/sub2api/tree/b0a02a3304a0a78cdd003ddda1dd23ba02c03084), root LICENSE: LGPL-3.0.

These are observations about engineering constraints, not imported implementations. No source, DTOs, generated contracts, dependencies, or assets from either repository are included. ModuAPI's contract and code are independently written. The project's own license is deliberately left for its owner to choose.

Phase 2 rechecked both GitHub HEADs on the same date; the pinned revisions remain
current. The scheduling constraints and explicit differences are documented in
[scheduling lessons](research/scheduling-lessons.md) and the
[scheduler design](design/scheduler.md). No upstream historical compatibility or
account/quota logic was transplanted.

## New API

Files read: `router/relay-router.go`, `middleware/distributor.go`, `middleware/body_cleanup.go`, `controller/relay.go`, `model/channel.go`, `model/channel_cache.go`, `service/channel.go`, `service/channel_affinity.go`, `common/body_storage.go`, `relay/channel/adapter.go`, `relay/channel/api_request.go`, `relay/channel/openai/adaptor.go`, `relay/helper/stream_scanner.go`, `relay/common/stream_status.go`, `relaykit/README.md`, `relaykit/dto/request_common.go`, `relaykit/relayconvert/convmeta/meta.go`.

- The router installs auth, limiters and channel distribution before relay. The controller coordinates protocol dispatch, channel selection, retry, billing and failure side effects. This is useful evidence for separating lifecycle from those business policies in ModuAPI.
- Channel selection groups eligible channels by priority, then chooses by weight inside a priority tier. Retry can descend tiers. All-zero weights and tiny weights need explicit semantics. Priority ordering differs across projects; the host must not silently impose a universal scoring formula.
- Channel metadata includes mapping, status and auto-disable state beside persistence/business fields. A scheduler needs eligibility facts, not a database channel entity. Model mapping belongs to a protocol adapter's preparation step.
- Body storage provides independent replay readers and bounded memory/disk ownership. `GetBody` must create a new reader; closing one attempt must not destroy the source of later attempts. This slice uses a capped immutable byte payload and fresh readers, postponing disk spooling.
- Upstream request construction uses request context, preserves body metadata, controls headers, and stops automatic redirects. ModuAPI similarly keeps credentials out of ingress passthrough and disables redirects in its HTTP plugin.
- Stream handling distinguishes transport termination from protocol completion; slow writes need their own deadline. Scanner size limits, goroutine joins, closing bodies before waiting, and explicit terminal events are correctness issues, not optional optimizations.
- Affinity has configurable extraction and cache state; skip-retry behavior can change availability. ModuAPI treats affinity as an opaque scheduler hint and supplies exclusions on every attempt. It does not hardcode session field names or Redis.
- RelayKit isolates conversion from transport and selection, but its DTO/context surface also carries protocol and host concerns. ModuAPI keeps only protocol/model/opaque payload in canonical requests; conversion state stays inside a protocol stream RPC.

## Sub2API

The requested `backend/internal/gateway/` directory is absent at this commit. Relevant implementation is in `backend/internal/handler/`, `backend/internal/service/`, `backend/internal/pkg/httpclient/` and `backend/pkg/pluginapi/v1/`.

Files read: `gateway_handler.go`, `openai_gateway_handler.go`, `gateway_handler_stream_failover_test.go` in handler; `gateway_scheduling.go`, `openai_account_scheduler.go`, `openai_sticky_compat.go`, `concurrency_service.go`, `gateway_forward.go`, `gateway_request.go`, `gateway_upstream_response.go`, `gateway_upstream_transport_error.go`, `gateway_usage_billing.go`, `openai_client_transport.go`, `plugin_runtime.go`, `plugin_manifest.go`, `openai_plugin_transport.go` in service; `pkg/httpclient/pool.go`; `pkg/pluginapi/v1/plugin.proto` and `runtime.go`. Large handler/service files were read by relevant function sections rather than as a claim of exhaustive audit.

- Account selection has multiple sticky/previous-response layers, excluded IDs, availability checks, bounded waiting and load-aware fallbacks. It rechecks eligibility before acquiring capacity. A hint is not an unconditional pin; a slot reservation needs release on every exit.
- Scheduler result reporting feeds error/latency state. ModuAPI exposes `Report` with the selected candidate, opaque lease and attempt result, alongside candidate metadata/in-flight facts. Distributed queues, health scores and account state are deferred to plugins.
- Session hash compatibility has migration logic and dual reads/writes. Hash derivation and namespace/version policy must remain with the plugin that owns session state.
- Shared clients reuse transports keyed by configuration. Recreating a transport per request defeats pooling. ModuAPI's HTTP plugin owns one pooled transport per process; it closes idle connections on shutdown.
- Transport failures distinguish client cancellation from upstream faults; cancellation must not trigger failover or health eviction. Returned errors need safe public messages rather than internal URLs/credentials.
- Stream code and regression tests guard against failover after client-visible output. Protocol terminal events and partial usage matter even when HTTP status is 200. ModuAPI tracks commitment and terminates partial streams without synthesizing success.
- Some Sub2API paths intentionally drain after a downstream disconnect to collect billing usage. ModuAPI chooses immediate cancellation, as required by its kernel contract; usage is best effort and explicitly partial.
- Plugin runtime uses independent processes/gRPC, checks identity/version/capabilities, validates configuration and drains in-flight calls. Its host services include accounts and credentials. ModuAPI's v1 has no account directory, database, UI bridge or billing host service.

## Consequences for this slice

Separate router, scheduler, retry policy and transport. Make all calls cancellable. Bound bodies, frame sizes, startup, first byte, idle reads, writes, request duration and shutdown. Preserve opaque protocol data on the common path; do not decode/re-encode the request on every hop. Require explicit opt-in to replay potentially billable POSTs. Stream through bounded gRPC flow control. Exercise real plugin processes against a local mock, including cancellations and crashes.

## Phase 3A validation evidence

The audit's remaining hypothesis was tested instead of inferred. A second,
independent Python Anthropic Messages reference process uses only generated
protobuf/gRPC bindings and passes through the unchanged Core with the generic
HTTP upstream and scheduler. This is composition evidence, not a claim that
Anthropic support itself is a ModuAPI feature.

The experimental `embedding` package lets an external Go package own the
listener and compose `ServeHTTP`. The test is in an external package and uses a
real request. It reduces assembly to four exported operations (`Start`,
`Ready`, `ServeHTTP`, `Close`), but it does not remove host responsibilities:
listener/TLS, configuration, deployment, credentials/authentication, and
application tests remain outside the Kernel. The API is not declared stable.
The actual consumer surface includes `core/config` and `core/router`, not just
four operations; see the [embedding API review](validation/embedding-api-review.md).

The Linux placement experiment compared split (Kernel + three processes),
bundle (Kernel + one multi-service process), local (in-process control), and a
direct mock. It measured total/first-byte/residual latency, RPS, CPU, RSS, FDs,
slow consumers, continuous cancellation, crash/restart and a 30-second mixed
run. The raw records and method are in [Phase 3A validation](validation/phase-3a.md).
The key result is a placement trade-off, not a universal advantage: bundle
roughly halves small-body process/RSS/FD costs relative to split, while large
bodies still amplify copies and buffering. The residual is the whole gateway
path; it cannot be honestly attributed to gRPC without an ablation that was not
performed.

This leaves a narrow, falsifiable value proposition: a trusted operator or
integrator that needs to release protocol, transport and selection components
independently across languages may save host coupling and lifecycle work. The
repository has not yet measured that saving against an independent adopter's
existing APISIX/ext_proc, Bifrost, llmgate, LiteLLM, New API or Sub2API design.
The [exit gate](validation/phase-3a-exit-gate.md) therefore remains
**CONDITIONAL GO**, with STOP as the correct outcome if that requirement does not
appear.

## External-demand closeout

The 2026-10-17 [public-source investigation](research/external-demand-validation.md)
records ten dated, attributed leads/counterexamples. It finds genuine separate
extension requests, but **no confirmed independent ModuAPI component author or
embedder**, no participant-owned savings measurement, and no source proving the
combined three-component/process requirement. Resolved Envoy/Bifrost/llm-d
cases are retained as counterevidence. All leads are UNVERIFIED; no messages
were sent. The [alternatives review](validation/phase-3a-alternatives.md)
distinguishes source facts, engineering estimates and unknown implementation costs.

[Author](validation/component-author-validation.md) and
[host](validation/embedding-host-validation.md) briefs include reproducible
examples, interview questions and a shared cost template. They prepare a trial;
they do not replace an external participant. The
[workload assessment](validation/phase-3a-workload-fit.md) documents where
external placement is a plausible fit and where its cost is material. Dedicated
native Linux is **BLOCKED**, so GO remains unavailable. Only bounded evidence
collection continues, with no feature or API development.
