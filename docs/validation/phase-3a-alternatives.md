# Same requirement, real alternatives

Date: 2026-10-17. ModuAPI baseline: `dfde6e6`. This is a source-verified
integration plan comparison, **not** completed implementations in competing
systems. There is no measured competitor LOC, time saving, latency or RSS in
this review. Unknowns are not filled with invented estimates.

## Fixed comparison task

For comparison only, suppose an integrator must (1) replace ingress parsing and
JSON/SSE response conversion, (2) implement complete candidate selection with
feedback, and (3) replace the upstream transport. Suppose the components have
independent language/release owners and the Go application owns HTTP serving.
These are **test requirements, not confirmed customer requirements**. Relaxing
the process/language condition often makes an existing solution preferable.

`F` = directly supported by inspected source/contract; `E` = engineering
inference/work inventory, not an implementation measurement; `U` = unknown
until the participant implements the same acceptance tests. References are
pinned to the [source audit](../research/competitive-architecture-audit.md),
except the newly pinned go-plugin sources below.

## Concrete integration paths

| Alternative | Source-verifiable path (F) | Glue to write/maintain for the fixed task (E) | Measured effort (U) |
|---|---|---|---|
| ModuAPI | Existing Lifecycle + Protocol/Upstream/Scheduler v1 services, manifest and process manager; experimental `embedding` Handler. | Implement component semantics; generate foreign-language bindings; lifecycle server if SDK unavailable; package manifests; host configuration/listener/drain; workload tests. Kernel supplies frame/retry/cancellation/reaping rules. | Existing reference artifacts are 292 Python protocol LOC, 123 conformance LOC and 52 embedding-test LOC. These exclude generated bindings, tooling and host operations. **Saved LOC/hours unknown**. |
| Bifrost core library | [`Init` / library lifecycle](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/core/bifrost.go) and [`BifrostConfig`](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/core/schemas/bifrost.go) support library composition. [`LLMPlugin` / `HTTPTransportPlugin`](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/core/schemas/plugin.go) provide typed hooks, including request routing mutation. | For known protocols: assemble the library and hook; likely no new process lifecycle code. For this *external-language* task: add an RPC bridge, foreign runtime supervision and error/cancellation mapping; map protocol data to Bifrost types or wrap with a custom HTTP handler. Whole transport replacement must be evaluated against [`Provider`](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/core/schemas/provider.go), not assumed to be one hook. | No Bifrost assembly or external bridge was built. LOC, development days and performance **unknown**. An in-process hook may already satisfy the participant with less work. |
| APISIX + external runner | [External plugin lifecycle](https://github.com/apache/apisix/blob/e14c6e9371edc084cc51321d4bee78646fa8367d/docs/en/latest/external-plugin.md) and [runner wire protocol](https://github.com/apache/apisix/blob/e14c6e9371edc084cc51321d4bee78646fa8367d/docs/en/latest/internal/plugin-runner.md): managed child, Unix socket RPC, phase execution and extra-info exchange. | Write phase plugins and route/upstream mappings. Evaluate body/response phase capabilities against the required SSE transformation. Complete transport/selection replacement is not a direct typed runner slot; it may need a Lua AI/balancer extension or external service. Do not assert a fork is always necessary. Multilingual independent runner management needs a current-version trial (D1 was APISIX 3.4). | Runner adaptation, Lua work and deployment effort **unknown**. No APISIX instance was run. |
| Envoy + ext_proc | [`ExternalProcessor.Process`](https://github.com/envoyproxy/envoy/blob/bc3dc71cdc6745df0f0e4df6f5c02d68e19a09a8/api/envoy/service/ext_proc/v3/external_processor.proto) is a language-neutral bidi stream. Header/body/trailer mutation, immediate responses and routing metadata are built in. | Implement one processor, processing mode/configuration, route/cluster mapping, and deploy/supervise it. Keep transport in Envoy if possible. If arbitrary transport is mandatory, operate a separate upstream proxy/service; ext_proc is not a replacement transport interface. Selection can return routing hints while Envoy owns actual routing. | Processor/config LOC, operational effort and mode-specific latency **unknown**. D5 demonstrates that some apparent gaps require only configuration. |
| Go host + go-plugin | [`ClientConfig`/version negotiation/process startup/kill](https://github.com/hashicorp/go-plugin/blob/fa60f8a91608db94647a9dc739b3a33634d35163/client.go), [cross-language support](https://github.com/hashicorp/go-plugin/blob/fa60f8a91608db94647a9dc739b3a33634d35163/README.md) and [`GRPCBroker`](https://github.com/hashicorp/go-plugin/blob/fa60f8a91608db94647a9dc739b3a33634d35163/grpc_broker.go) are existing mechanisms. | Reuse startup/transport/versioning; define application protocol + three interfaces, stream/terminal/error rules, request/retry/commit ownership, routing, reports/leases, configuration/manifest policy, supervisor/restart policy and contract tests. Could instead expose **one coarser service** and avoid ModuAPI's six business RPCs. | Bespoke gateway glue and ongoing compatibility cost **unknown**. go-plugin is not an empty starting point; do not count its lifecycle code as work an integrator must rewrite. |

## Costs beyond source lines

| Dimension | ModuAPI | Bifrost | APISIX / Envoy | go-plugin host |
|---|---|---|---|---|
| Dependencies / deployment | **F:** Go YAML/gRPC/protobuf plus installed executables/interpreters. Minimal split = 4 processes, bundle = 2. No required business DB. | **F:** Go library with schema/provider dependencies; DB/UI not required merely to embed core. Native hooks share the process. External bridge adds services (**E**). | **F:** proxy distribution/configuration plus runner/processor. Existing APISIX/Envoy operators may have little incremental infrastructure (**E**); greenfield users have more (**E**). | **F:** Go library and gRPC child/foreign runtime; application adds gateway stack. Service count depends on its chosen contract (**U**). |
| Host type dependency | **F:** v1 wire DTOs and namespaced metadata; embedding also exposes `core/config` and `core/router`. | **F:** `BifrostContext`, request/response types, and [`Account`](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/core/schemas/account.go) provider/key/config methods. Account is an interface, **not a forced database/user system**. | **F:** HTTP filter/runner types, route/cluster concepts; no AI business object required by ext_proc. | **F:** generic client/plugin interfaces; application owns all domain/wire types. |
| Upgrade compatibility | **F:** additive wire policy/conformance; little release history, experimental embedding; mixed-version compatibility evidence limited. | **E:** pin Go modules, recompile/test hooks and conversions as types evolve. [Inspected `.so` loader](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/framework/plugins/soloader.go) uses Go `plugin.Open`; it is not a language-neutral process contract. | **F:** versioned external contracts. **E:** pin proxy/runner modes and run application compatibility tests; maturity does not eliminate config/semantic changes. | **F:** versioned PluginSets; application still versions message/semantic contracts. |
| Lifecycle / errors | **F:** startup identity/config/health checks, bounded supervision; cancellation/stream commitment/retry safety supplied. Report is best effort; scheduler state resets on restart. | **F:** library lifecycle and typed errors/hooks. **E:** out-of-process bridge would add ownership and crash semantics; in-process avoids that work. | **F:** APISIX manages runner; Envoy calls a configured processor service rather than spawning it. [Envoy failure/timeout policy](https://github.com/envoyproxy/envoy/blob/bc3dc71cdc6745df0f0e4df6f5c02d68e19a09a8/api/envoy/extensions/filters/http/ext_proc/v3/ext_proc.proto) is explicit. | **F:** startup/connection/kill framework; application decides readiness, restart budget, HTTP retry/terminal semantics. |
| Debugging / operations | **F:** multiple PIDs, bounded restricted stderr, Go/Python traces across RPC; limited diagnostics. **E:** independent failure boundary can help or hurt depending on the team. | **E:** one-language call stacks easier for Go team; panic shares host. Foreign bridge would remove that advantage. | **E:** filter order, body modes, route cache, proxy logs and external-service logs must be correlated. Existing operators may prefer familiar tools. | **E:** broker/child/application logs and application wire protocol require correlation; D4 shows cross-language callback friction. |
| Performance loss | **F:** WSL placement data below; six business RPCs, body and response serialization. **U:** native dedicated-host cost. | **F:** native hook path has no added process RPC; **U:** comparable latency/CPU/RSS. Do not reuse vendor microbenchmarks as this workload's result. | **F:** mode/phase-dependent RPC and body copying. [Envoy modes](https://github.com/envoyproxy/envoy/blob/bc3dc71cdc6745df0f0e4df6f5c02d68e19a09a8/api/envoy/extensions/filters/http/ext_proc/v3/processing_mode.proto) include streamed/buffered/full duplex. **U:** equivalent-workload overhead. | **F:** RPC/serialization for external calls. **U:** latency depends on coarser/finer application contract; cannot infer it from ModuAPI. |

## What can actually be claimed as a saving?

With go-plugin, ModuAPI offers **application-level semantics and tests on top of
commodity process RPC**. Against APISIX/Envoy it offers typed full-component
slots, at the cost of a younger runtime and less mature proxy operations.
Against Bifrost it offers a foreign-process contract instead of native typed
hooks. None of those facts establishes that a participant needs the difference.

The task inventory identifies possible avoided contracts: startup negotiation,
frame ordering/terminal validation, cancellation propagation, stream commitment,
retry exclusions, process reaping/recovery and report/lease conventions. The
participant still writes protocol/policy/transport logic and integration tests.
**Avoided contract count is not measured avoided development work**: the
alternative may already supply enough of it or may not need a given contract.

Actual extra work without ModuAPI is therefore **UNKNOWN**. Populate the
[cost record](integration-cost-record.md) from a participant's selected best
alternative, including maintained code, generated code, debug time and upgrades.
Do not compare ModuAPI to a deliberately hand-built weak proxy, the whole LOC
of a mature gateway, or a hypothetical complete reimplementation of go-plugin.
For current workload limits see [workload fit](phase-3a-workload-fit.md).
