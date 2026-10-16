# Architecture due diligence: is ModuAPI worth continuing?

Date: 2026-10-16. Baseline: **a3f011d405785886a5c723979d28b9ec78f61739**.
Decision: **conditional Go for positioning validation, No-Go for a general-purpose
gateway feature roadmap**. See [ADR 0009](../adr/0009-moduapi-positioning.md).
No new plugin kinds, production API changes or production architecture changes
are part of this audit. Research is source inspection, not a production deployment
or security certification of the compared projects.

## Method and reproducible evidence

Repository trees and default-branch heads were retrieved through GitHub's API.
The links below pin the actual source reviewed; `dev` is deliberately identified
for Bifrost. These are snapshots, not necessarily the projects' stable releases.
Large service files were inspected at the relevant interfaces, implementations and
call sites. Absence claims concern the inspected extension boundary, not every
possible integration somebody could build around a project. No upstream source,
protobuf, adapter implementation or benchmark claim was imported into ModuAPI.

| Project | Inspected commit | Main source evidence and concrete responsibilities |
|---|---|---|
| ModuAPI | `a3f011d405785886a5c723979d28b9ec78f61739` | M1: [gateway lifecycle](../../core/gateway/gateway.go), router/retry/request/streaming/config and their tests; M2: [process](../../runtime/process/process.go), restart/stderr, manifest/version, registry, RPC adapters; M3: [proto](../../proto/moduapi/plugin/v1/plugin.proto), SDK types/extensions/stream, Go server; M4: [scheduler](../../plugins/official/scheduler-priority-weight-health/scheduler.go), config, tests/benchmarks; M5: [protocol](../../plugins/official/protocol-openai/protocol.go), upstream-http, single/roundrobin; M6: tests/integration, tests/conformance/python and Go contract tests, architecture import test; M7: all existing ADRs, architecture/contract/developer guide/research/lessons/roadmap; M8: [composition](../../internal/app/app.go), buildutil, CLI/config/examples. Repository-relative references remain unchanged in this audit. |
| New API | `d032204733db31208f10bac118b6e683aab3bf29` (`main`) | N1: [Task Plugin v1 specification and TypeScript contract](https://github.com/QuantumNous/new-api/tree/d032204733db31208f10bac118b6e683aab3bf29/docs/plugin-api); N2: [JS engine](https://github.com/QuantumNous/new-api/blob/d032204733db31208f10bac118b6e683aab3bf29/pkg/jsplugin/engine.go), registry/routing/request/fetch alongside it; N3: [task adaptor](https://github.com/QuantumNous/new-api/blob/d032204733db31208f10bac118b6e683aab3bf29/relay/channel/task/jsplugin/adaptor.go), submit_stream.go; N4: [channel cache selection](https://github.com/QuantumNous/new-api/blob/d032204733db31208f10bac118b6e683aab3bf29/model/channel_cache.go), service/channel_affinity.go, controller/relay.go; N5: [plugin controller](https://github.com/QuantumNous/new-api/blob/d032204733db31208f10bac118b6e683aab3bf29/controller/task_plugin.go), model/task_plugin.go, middleware/task_plugin.go. |
| Sub2API | `b0a02a3304a0a78cdd003ddda1dd23ba02c03084` (`main`) | S1: [wire contract](https://github.com/Wei-Shaw/sub2api/blob/b0a02a3304a0a78cdd003ddda1dd23ba02c03084/backend/pkg/pluginapi/v1/plugin.proto), runtime.go, manifest.schema.json; S2: [host runtime](https://github.com/Wei-Shaw/sub2api/blob/b0a02a3304a0a78cdd003ddda1dd23ba02c03084/backend/internal/service/plugin_runtime.go), plugin_manifest.go/manager.go/host_services.go, openai_plugin_transport.go; S3: [account scheduler](https://github.com/Wei-Shaw/sub2api/blob/b0a02a3304a0a78cdd003ddda1dd23ba02c03084/backend/internal/service/openai_account_scheduler.go), gateway_scheduling.go/concurrency_service.go; S4: [package and security contracts](https://github.com/Wei-Shaw/sub2api/tree/b0a02a3304a0a78cdd003ddda1dd23ba02c03084/backend/pkg/pluginapi/docs); S5: [usage/billing ownership](https://github.com/Wei-Shaw/sub2api/blob/b0a02a3304a0a78cdd003ddda1dd23ba02c03084/backend/internal/service/gateway_usage_billing.go). The requested `backend/internal/gateway` directory does not exist at this revision. |
| Apache APISIX | `e14c6e9371edc084cc51321d4bee78646fa8367d` (`master`) | A1: [runner protocol](https://github.com/apache/apisix/blob/e14c6e9371edc084cc51321d4bee78646fa8367d/docs/en/latest/internal/plugin-runner.md), external-plugin.md; A2: [external RPC/process implementation](https://github.com/apache/apisix/blob/e14c6e9371edc084cc51321d4bee78646fa8367d/apisix/plugins/ext-plugin/init.lua), helper.lua; A3: [post-response extension](https://github.com/apache/apisix/blob/e14c6e9371edc084cc51321d4bee78646fa8367d/apisix/plugins/ext-plugin-post-resp.lua), pre/post-request plugins and apisix/plugin.lua; A4: [AI multi-upstream proxy](https://github.com/apache/apisix/blob/e14c6e9371edc084cc51321d4bee78646fa8367d/apisix/plugins/ai-proxy-multi.lua), ai-proxy.lua/base.lua. |
| Envoy ext_proc | `bc3dc71cdc6745df0f0e4df6f5c02d68e19a09a8` (`main`) | E1: [external processor protocol](https://github.com/envoyproxy/envoy/blob/bc3dc71cdc6745df0f0e4df6f5c02d68e19a09a8/api/envoy/service/ext_proc/v3/external_processor.proto); E2: [processing modes](https://github.com/envoyproxy/envoy/blob/bc3dc71cdc6745df0f0e4df6f5c02d68e19a09a8/api/envoy/extensions/filters/http/ext_proc/v3/processing_mode.proto) and ext_proc.proto filter configuration; E3: [filter](https://github.com/envoyproxy/envoy/blob/bc3dc71cdc6745df0f0e4df6f5c02d68e19a09a8/source/extensions/filters/http/ext_proc/ext_proc.cc), processor_state.cc, ext_proc_filter.rst. |
| Bifrost | `767bd89a639efbe1461d7a653f451124ecd23931` (`dev`) | B1: [plugin interfaces and ordering](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/core/schemas/plugin.go), provider.go/context.go; B2: [core library](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/core/bifrost.go), sessionaffinity.go; B3: [plugin loaders](https://github.com/maximhq/bifrost/tree/767bd89a639efbe1461d7a653f451124ecd23931/framework/plugins), transports/bifrost-http/server/plugins.go; B4: [routing framework](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/framework/routing/routing.go). |
| llm-d EPP | `26200ef850868884fcbe7aaf604d2f3c508c3cae` (`llm-d/llm-d-router`, `main`) | D1: [scheduling interfaces](https://github.com/llm-d/llm-d-router/blob/26200ef850868884fcbe7aaf604d2f3c508c3cae/pkg/epp/framework/interface/scheduling/plugins.go), types.go; D2: [scheduler/profile executor](https://github.com/llm-d/llm-d-router/tree/26200ef850868884fcbe7aaf604d2f3c508c3cae/pkg/epp/scheduling); D3: [ext_proc server](https://github.com/llm-d/llm-d-router/blob/26200ef850868884fcbe7aaf604d2f3c508c3cae/pkg/epp/handlers/server.go), request.go/response.go, requestcontrol/director.go; D4: [framework interfaces](https://github.com/llm-d/llm-d-router/tree/26200ef850868884fcbe7aaf604d2f3c508c3cae/pkg/epp/framework/interface), especially requesthandling.Parser, plugin.Handle/registry/dependencies; D5: [architecture](https://github.com/llm-d/llm-d-router/blob/26200ef850868884fcbe7aaf604d2f3c508c3cae/docs/architecture.md) and session-affinity filter/scorer implementations under framework/plugins/scheduling. This is the current EPP source; older inference-scheduler paths should not be assumed. |
| llmgate | `6fbd800f4aefb18a647d8ff8b931204c51ecbe63` (`devstroop/llmgate`, `main`) | G1: [neutral model](https://github.com/devstroop/llmgate/blob/6fbd800f4aefb18a647d8ff8b931204c51ecbe63/src/core/neutral.rs); G2: [ProtocolAdapter and stream traits](https://github.com/devstroop/llmgate/blob/6fbd800f4aefb18a647d8ff8b931204c51ecbe63/src/core/registry.rs); G3: [pipeline](https://github.com/devstroop/llmgate/blob/6fbd800f4aefb18a647d8ff8b931204c51ecbe63/src/core/pipeline.rs), proxy.rs/config.rs/auth.rs/lib.rs and OpenAI adapter. The project identity is explicit because multiple repositories use similar names. |
| LiteLLM | `2dc6aafa6baf8dabdac58f5ba70c384b8dcb3e09` (`main`) | L1: [Router](https://github.com/BerriAI/litellm/blob/2dc6aafa6baf8dabdac58f5ba70c384b8dcb3e09/litellm/router.py), types/router.py; L2: [callback contract](https://github.com/BerriAI/litellm/blob/2dc6aafa6baf8dabdac58f5ba70c384b8dcb3e09/litellm/integrations/custom_logger.py); L3: [least-busy strategy](https://github.com/BerriAI/litellm/blob/2dc6aafa6baf8dabdac58f5ba70c384b8dcb3e09/litellm/router_strategy/least_busy.py), lowest_latency.py, caching/dual_cache.py; L4: [proxy composition](https://github.com/BerriAI/litellm/blob/2dc6aafa6baf8dabdac58f5ba70c384b8dcb3e09/litellm/proxy/proxy_server.py). |

Root license files reviewed at those commits: New API AGPL-3.0; Sub2API LGPL-3.0;
APISIX/Envoy/Bifrost/llm-d-router Apache-2.0; llmgate MIT; LiteLLM root MIT with
explicit directory-specific exceptions including enterprise/. This is an inventory,
not a conclusion about obligations for hypothetical integrations. ModuAPI still
has no owner-selected LICENSE. No copying/derivation was used to avoid that choice.

## Comparison matrix

Every cell is relative to the **ModuAPI baseline**, and includes evidence IDs from
the inventory above. `same` means the particular mechanism/ownership matches;
`similar` means the goal matches with a different contract; `different` is a scope
or boundary difference, not a quality score. `stronger`/`weaker` mean demonstrably
more/less support for that particular dimension, not an overall project ranking.
Runtime cost cells compare mechanisms only: **no competing gateway was benchmarked**.
An unimplemented ModuAPI feature is not counted as a plugin capability.

### Responsibility and component shape

| Dimension | ModuAPI | New API | Sub2API | APISIX | Envoy ext_proc | Bifrost | llm-d EPP | llmgate | LiteLLM |
|---|---|---|---|---|---|---|---|---|---|
| 1 Core responsibility | same: AI HTTP lifecycle (M1) | different: relay + business/task platform (N1,N4) | different: account/business gateway (S2,S3) | different: generic proxy + Lua plugins (A2,A4) | different: HTTP proxy + remote filter (E1,E3) | different: provider-aware AI library/gateway (B2) | different: inference endpoint control (D3,D5) | different: translation/privacy gateway (G3) | different: provider SDK/router + proxy (L1,L4) |
| 2 Protocol abstraction | same: opaque envelope, external Protocol (M3,M5) | similar: host protocols + native routes (N1,N3) | different: host protocol, transport extension (S1,S2) | different: HTTP phases; AI adapters in plugins (A3,A4) | different: generic HTTP messages (E1) | different: typed LLM/MCP requests (B1) | similar: replaceable Parser, inference body (D4) | stronger: neutral content + cross-protocol adapters (G1,G2) | different: provider transformations, common AI types (L1,L2) |
| 3 Upstream abstraction | same: replaceable Execute stream (M3) | similar: task driver + channel relay (N3,N4) | similar: independent Forward transport (S1) | similar: upstream/balancer + AI proxy (A4) | different: Envoy cluster/router retains transport (E1,E3) | similar: Provider methods, built-in integrations (B1,B2) | different: proxy owns transport; EPP selects endpoints (D3) | different: shared reqwest + adapter URLs (G2,G3) | similar: model deployments/provider adapters (L1) |
| 4 Scheduler abstraction | same: whole Select/Report replacement (M3) | different: host channel selector (N4) | similar: account scheduler interface in host (S3) | similar: balancers/AI instance selection (A4) | different: proxy LB; no AI scheduler SPI in ext_proc (E1) | similar: routing/key selection in library/framework (B2,B4) | stronger: Filter/Score/Pick/Profile composition (D1,D2) | weaker: resolver; no comparable scheduler service (G3) | similar: strategies + custom routing hooks (L1,L3) |
| 5 Plugin granularity | same: three capability slots (M2,M3) | different: task driver/protocol/native-route module (N1) | different: outbound transport capability (S1,S2) | different: phase/filter plugins, runner batches (A1,A3) | different: request/response processor in filter chain (E1) | different: LLM/HTTP/MCP hooks (B1) | different: small framework extension points (D1,D4) | different: Rust protocol trait (G2) | different: Python callbacks/strategies/providers (L1,L2) |

### Runtime contract

| Dimension | ModuAPI | New API | Sub2API | APISIX | Envoy ext_proc | Bifrost | llm-d EPP | llmgate | LiteLLM |
|---|---|---|---|---|---|---|---|---|---|
| 6 Process boundary | same: operator-installed child + gRPC (M2) | different: in-process JS VM pool (N2) | same: child + gRPC; go-plugin bootstrap (S1,S2) | similar: child runner + Unix/FlatBuffers RPC (A1,A2) | similar: external gRPC service, not spawned by filter (E1,E2) | different: Go interfaces/.so loader inspected (B1,B3) | different: EPP external to proxy; plugins in EPP (D3,D4) | different: in-process Rust traits (G2) | different: in-process Python extensions (L2) |
| 7 Cross-language | same: Go/Python process conformance (M6) | weaker: JS task-module language (N2) | similar: protobuf + go-plugin framing, Go helper (S1) | stronger: multiple documented language runners (A1) | stronger: public language-neutral external service (E1) | different: Go hook ABI; WASM schema scaffolding not deployment-verified (B1,B3) | weaker: Go extension interfaces; gRPC is proxy-facing (D1,D3) | weaker: Rust trait contract (G2) | weaker: Python callback API (L2) |
| 8 Lifecycle | same: negotiate/configure/ready/drain/restart (M2) | different: compile/enable/atomic routing generations (N2,N5) | similar: identity/config/health/drain/kill (S2) | similar: runner spawn/reload/TERM/KILL (A2) | different: per-request streams; deployment owns server (E1,E3) | different: Init/Cleanup and loader management (B1,B3) | different: factory registration/readiness + framework lifecycle (D4,D5) | weaker: construct/register with service (G2,G3) | different: callback/proxy startup/shutdown (L2,L4) |
| 9 Manifest | same: strict YAML bundle manifest (M2) | similar: JS meta + schema/compiled validation (N1,N2) | stronger: platform runtimes/files/signature package (S2,S4) | different: runner command + route configuration (A1,A3) | different: protobuf/xDS filter config (E2) | different: plugin path/config + implemented interfaces (B3) | different: typed config/factories/profiles (D4,D5) | different: TOML config + registered adapters (G2,G3) | different: YAML/Python callback/import config (L2,L4) |
| 10 Capability negotiation | same: flat required feature intersection (M2,M3) | different: validated host protocol/mode claims (N1,N2) | similar: capability/platform/account scope + API versions (S1,S2) | weaker: runner RPC/version conventions, no same handshake (A1,A2) | different: processing modes/protocol configuration per stream (E1,E2) | different: optional Go hook symbols/interfaces (B1,B3) | different: interface/dependency registration (D4) | weaker: trait implementation, no runtime negotiation (G2) | weaker: callback/strategy contracts, no process negotiation (L2) |

### Request and stream semantics

| Dimension | ModuAPI | New API | Sub2API | APISIX | Envoy ext_proc | Bifrost | llm-d EPP | llmgate | LiteLLM |
|---|---|---|---|---|---|---|---|---|---|
| 11 Configuration validation | same: object + plugin Configure; schema not fully enforced (M2,M3) | stronger: compiled meta/schema/route conflict checks (N1,N2,N5) | stronger: ValidateConfig/normalized ApplyConfig/TestConfig (S1,S2) | similar: plugin schemas + PrepareConf token (A1,A3) | stronger: typed validated protobuf/filter configuration (E2) | similar: plugin Init/config decoding, optional hooks (B3) | stronger: factory/profile/dependency validation (D2,D4) | similar: typed config and startup checks (G3) | similar: Pydantic/config/proxy setup (L1,L4) |
| 12 Ordering | same: fixed stages; ordered local hooks, no plugin pipeline (M1,M3) | different: protocol/native bindings and key ordering (N1,N2) | different: capability binding selects transport (S2) | stronger: host priorities + ordered runner conf (A1,A3) | stronger: filter chain + processing modes (E2,E3) | stronger: pre registration order/post reverse order (B1) | stronger: profiles + producer/consumer ordering (D2,D4) | different: fixed pipeline/adapter registry (G2,G3) | different: callback stages and routing hooks (L1,L2) |
| 13 Request context | same: context + opaque bytes/metadata (M1,M3) | different: task/channel/usage/driver context (N1,N3) | different: HTTP + account/platform/proxy metadata (S1) | similar: request context + ExtraInfo callbacks (A1,A2) | similar: headers/body/attributes/dynamic metadata (E1) | different: mutable BifrostContext + typed AI requests (B1) | different: inference body/objectives/endpoints/shared attributes (D1,D4) | different: NeutralRequest content blocks (G1) | different: common AI kwargs + request/auth metadata (L1,L2) |
| 14 Streaming | same: Execute server stream + Transform bidi; terminal/EOF guard (M3) | similar: stream event parser/render hooks driven by host (N1,N3) | similar: bidi start/chunk/end/error forwarding (S1,S2) | different: phase RPC; optional whole response-body fetch (A2,A3) | stronger: selectable streamed/buffered/full-duplex HTTP bodies (E1,E2) | similar: typed stream channels + chunk hooks (B1,B2) | similar: ext_proc response parser and hooks (D3,D4) | similar: neutral stream decoder/encoder (G1,G2) | similar: iterators/generators + stream hooks (L1,L2) |
| 15 Cancellation | same: HTTP→RPC→upstream; joined sender (M1,M2,M6) | similar: context/VM interruption and host fetch (N2,N3) | similar: Forward context/body-close cancellation (S2) | different: socket timeouts/phase lifetime (A2,A3) | similar: onDestroy cancels processor stream (E3) | similar: BifrostContext/worker delivery ownership (B1,B2) | similar: gRPC stream context controls handler (D3) | similar: async stream ownership/drop and idle timeout (G3) | similar: async request/stream lifecycle (L1,L4) |

### Feedback and state

| Dimension | ModuAPI | New API | Sub2API | APISIX | Envoy ext_proc | Bifrost | llm-d EPP | llmgate | LiteLLM |
|---|---|---|---|---|---|---|---|---|---|
| 16 Backpressure | same: synchronous bounded frames, gRPC windows (M2,M3) | similar: host stream reader/writer + VM admission (N2,N3) | similar: gRPC + io.Pipe body ownership (S2) | different: socket/HTTP reader; RespBody may buffer whole body (A2,A3) | stronger: body modes + filter high/low watermarks (E2,E3) | similar: bounded provider workers/channels (B2) | similar: ext_proc + flow-control/admission framework (D3,D5) | similar: Rust async stream pulls (G3) | similar: async generators; buffering varies by adapter (L1,L4) |
| 17 State ownership | same: process-local policy; host generic in-flight (M1,M4) | different: host DB/tasks/channels + VM modules (N1,N4,N5) | different: account services + plugin state + host KV (S1,S2) | different: Nginx workers/shared dict + runner cache (A1,A2) | different: proxy per-stream + external-service state (E1,E3) | different: library/provider/plugin/framework layers (B2,B4) | different: EPP datastore + plugin shared/request state (D3,D4) | different: AppState + optional memory store (G3) | different: router/cache/proxy callback state (L1,L3,L4) |
| 18 Health ownership | same: candidate health in scheduler; process readiness in runtime (M2,M4) | different: host channel state/selection (N4) | different: host account runtime stats/eligibility (S3) | similar: balancer/healthcheck modules, AI plugin (A4) | different: proxy clusters + external service deployment (E2,E3) | different: provider/key/routing layers (B2,B4) | similar: data-layer metrics and selection plugins (D1,D5) | weaker: no equivalent feedback scheduler in inspected adapter boundary (G2,G3) | similar: router cooldown + strategies (L1,L3) |
| 19 Sticky ownership | same: scheduler opaque HMAC/LRU; protocol supplies hint (M4,M5) | similar: host affinity extraction/cache + channel selection (N4) | similar: scheduler/session/account compatibility rules (S3) | similar: hash balancer and optional plugin policy (A4) | different: proxy LB/external policy, not intrinsic ext_proc (E1) | similar: library session-affinity/provider selection (B2) | similar: session-affinity filters/scorers (D5) | weaker: no equivalent scheduler binding service (G2,G3) | similar: router deployment/encrypted-content affinity (L1) |
| 20 Retry ownership | same: kernel safety + policy, scheduler exclusions (M1) | different: host relay/channel retry selection (N4) | different: handler/service/account failover (S3) | different: proxy balancer/AI proxy retry (A4) | different: proxy route/retry/filter semantics (E1,E3) | different: library provider/fallback policy (B2) | different: proxy retry vs EPP selection/control (D3,D5) | weaker: no comparable independent replay policy (G3) | different: Router retries/fallbacks; guards emitted content (L1) |

### Business and trust boundary

| Dimension | ModuAPI | New API | Sub2API | APISIX | Envoy ext_proc | Bifrost | llm-d EPP | llmgate | LiteLLM |
|---|---|---|---|---|---|---|---|---|---|
| 21 Auth ownership | same: external edge required; no runnable auth plugin (M1,M3) | different: host authenticates plugin/native routes (N1,N5) | different: host identity/account/OAuth model (S1,S2) | different: optional auth plugins in proxy phases (A3) | different: separate filters/control-plane policy (E1,E2) | different: transport/governance hooks and provider keys (B1,B3) | similar: edge/proxy concern, not scheduler core (D3) | different: gateway auth module (G3) | different: proxy user/API-key/custom auth (L4) |
| 22 Usage ownership | same: protocol extraction; kernel carries token summary (M1,M3,M5) | different: driver facts validated/settled by task host (N1,N3) | different: gateway services/business usage (S5) | different: optional AI/observability plugins (A4) | different: external processor/filter telemetry (E1) | different: typed provider responses and plugins (B1,B2) | similar: Parser extracts usage, response hooks consume (D4) | similar: NeutralUsage + optional request record (G1,G3) | different: adapter/logging/proxy usage pipeline (L2,L4) |
| 23 Billing ownership | same: absent, experimental sink is not billing (M3,M7) | different: host pricing/task settlement (N1) | different: host business/account services (S5) | similar: not an external-runner primitive (A1) | same: absent from ext_proc contract (E1) | different: accounting/governance framework integration (B2,B3) | same: absent from scheduling contract (D1) | similar: not in neutral adapter contract (G1,G2) | different: proxy spend/budget/business integration (L1,L4) |
| 24 Persistence | same: none required/implemented (M1,M2) | different: host plugin/task/channel records (N5) | different: installation/config/account DB + host KV (S1,S2) | different: configured control-plane/config store; runner TTL cache (A1,A2) | similar: no app DB required by filter protocol (E1,E2) | similar: core library usable independently; framework stores optional layers (B2,B3) | different: Kubernetes discovery/state; no billing DB in scheduler (D3,D5) | similar: optional embedded memory store (G3) | similar: SDK/local router can run without DB/Redis; proxy adds stores (L1,L3,L4) |
| 25 Security isolation | same: trusted OS-user processes; grants cooperative (M2,M7) | different: restricted JS VM/host-mediated fetch, not hostile-code certification (N1,N2) | similar: same-user process; stronger bundle integrity, no OS sandbox (S2,S4) | similar: runner same OS user; not sandbox (A1,A2) | different: service deployment isolation; filter does not confine server (E1,E2) | different: native .so shares host address space; other paths not verified (B3) | different: Go plugins share EPP process (D4) | different: Rust modules share process (G2) | different: Python callbacks share proxy process (L2) |

### Product and operating cost

| Dimension | ModuAPI | New API | Sub2API | APISIX | Envoy ext_proc | Bifrost | llm-d EPP | llmgate | LiteLLM |
|---|---|---|---|---|---|---|---|---|---|
| 26 Distribution | same: local bundles/build scripts; no project LICENSE yet (M2,M8) | stronger: marketplace/source hash/version management (N5) | stronger: signed platform packages/installations (S2,S4) | different: host + language runner + plugin config (A1) | different: proxy/filter config + deployed processor service (E2) | stronger: library, gateway and dynamic loader packaging (B2,B3) | different: Kubernetes deployment + compiled plugin registry (D4,D5) | different: Rust library/binary + registered adapters (G2,G3) | stronger: Python SDK/proxy ecosystem (L1,L4) |
| 27 Runtime overhead | same: 6 request RPCs; measured separately below (M1,M2) | different: pooled VM/host conversion, no subprocess per hook (N2) | different: one bidi forwarding RPC for plugin transport (S1,S2) | different: phase RPC + optional ExtraInfo exchanges (A1,A2) | different: one bidi stream, mode-dependent message waits/copies (E1,E2) | different: in-process hooks; provider workers/queues (B1,B2) | different: proxy↔EPP stream; internal scheduling calls (D1,D3) | different: in-process conversion/async HTTP (G2,G3) | different: Python in-process hooks + router/cache overhead (L1,L2) |
| 28 Operational complexity | same: 4 minimal service processes; stock bundle directory starts 6 (M2,M8) | different: business gateway + VM plugin administration (N1,N5) | different: business services + outbound plugin packages (S2,S4) | different: proxy workers/config + external runners (A1,A2) | different: proxy/control configuration + external service (E2) | different: one core library or gateway/framework deployment (B2,B3) | different: proxy + EPP + Kubernetes/model metrics (D3,D5) | different: single gateway/library, optional memory/privacy (G3) | different: SDK or proxy/workers; optional stores (L1,L4) |
| 29 Extensibility | same: replace whole typed slots; not arbitrary stage graph (M2,M3) | different: task/native/protocol hooks within host model (N1) | different: capability transport plus optional host callback API (S1) | stronger: mature phase ordering/external-language runners (A1,A3) | stronger: generic HTTP processing/filter composition (E1,E2) | stronger: many hooks/provider library; domain types remain host-owned (B1,B2) | stronger: internal sub-policy and data dependency composition (D1,D4) | different: add Rust protocol adapter; semantic model evolves (G1,G2) | stronger: broad callbacks/strategies/integrations within AI SDK (L1,L2) |
| 30 What host knows | same: HTTP/model/stream/token summaries/candidates/stage order (M1,M3) | different: channels/tasks/protocols/users/prices/persistence (N1,N4,N5) | different: accounts/platform/OAuth/concurrency/credentials (S1,S2,S3) | different: HTTP/routes/upstreams/phases; optional AI plugins add domain (A3,A4) | different: HTTP streams/filter chain/clusters/metadata (E1,E2) | different: providers/keys/typed AI/MCP operations (B1,B2) | different: inference pools/endpoints/metrics/objectives/model bodies (D1,D3,D4) | different: chat content/tools/privacy/model resolution (G1,G3) | different: providers/models/common AI objects/retry/cost/proxy identity (L1,L2,L4) |

## What the source changes about our assumptions

**New API is not a non-extensible monolith.** Task plugins own native routes and
claim host-owned protocols/modes. The VM engine has admission limits, cancellation
watchdogs and pooled runtimes. Registry generations publish routing atomically;
the controller preflights conflicts, persists source/hash/revision and manages
marketplace sources. Usage schemas feed host settlement. ModuAPI differs in what
can be replaced and what the host must know, not in having invented plugins (N1–N5).

**Sub2API is the closest process-runtime overlap.** Its TransportPlugin already
has GetInfo, health, validation/application/testing and bidi Forward. The host
uses HashiCorp go-plugin, identity/version checks, checksums and bounded draining.
An optional InitHostServices callback explicitly tolerates UNIMPLEMENTED. Its
HostService includes scoped KV, account enumeration and outbound identity.
Those account/platform/credential fields are an intentional product boundary,
not an engineering defect. ModuAPI removes that mandatory domain from its wire API
and exposes selection as a replacement service (S1–S4).

**APISIX already provides external multi-language extensions.** PrepareConf tokens
avoid retransmitting configuration; a runner can ask for host variables/body through
ExtraInfo. Plugin order is explicit. Its external post-response path can fetch a
whole response body; this is not ModuAPI's per-frame Transform contract. APISIX also
already has AI multi-upstream priority/weight/health selection. Replacing a mature
HTTP proxy requires a concrete embedding/contract need, not fewer configuration
examples. We do not claim APISIX universally requires an application DB or that
external plugins are its only extension mechanism (A1–A4).

**Envoy ext_proc is a serious alternative to owning a data plane.** One Process
bidi stream carries selected headers/bodies/trailers and mutations. NONE/BUFFERED/
BUFFERED_PARTIAL/STREAMED/FULL_DUPLEX_STREAMED let operators choose the copy/wait
trade-off. Failure policy, message timeout, body chunk limits and watermark flow
control are explicit. Filter destruction cancels the external stream. New window
update messages marked `not-implemented-hide` in the inspected proto are **not**
counted as implemented flow-control features. gRPC alone does not make ModuAPI's
stream machinery more mature than Envoy's. Envoy does not spawn/negotiate our three
component services, but an external controller can supply lifecycle (E1–E3).

**Bifrost already separates a usable core library from its framework.** Its core
imports providers and uses typed AI requests, keys, worker queues and fallback
logic. LLMPlugin, HTTPTransportPlugin and MCPPlugin are distinct interception
surfaces with documented per-request/per-attempt ordering; newer optional hooks
make backwards-compatibility costs visible. The inspected dynamic loader uses
Go plugin.Open and optional symbols. WASM-related schema files/comments exist,
but this audit did not establish a deployed alternative loader and therefore does
not claim all Bifrost extensions are Go-only or that WASM is production-verified.
Its framework DB/governance/UI are not proof that its library mandates them (B1–B4).

**llm-d EPP's scheduling abstraction is more composable than ours.** Profiles run
filters, weighted scorers and a picker; a ProfileHandler chooses profiles and
combines results. Parsers claim paths/protocols and parse response usage. Request
control/data-layer extension points declare producer/consumer dependencies and
share typed state through a Handle. EPP is itself an ext_proc server; its scheduling
plugins are Go interfaces inside EPP, not independent RPC processes. ModuAPI's
Select/Report can *contain* such an algorithm but cannot compose those sub-policies
through the host. Our distinction is opaque candidates and replacement of a whole
policy outside a Kubernetes/inference-engine model. Weighting, affinity and load
selection are emphatically not new algorithms (D1–D5).

**llmgate already has protocol-neutral adapters.** NeutralRequest/Response contain
messages, tools, thinking/image/tool-use blocks and neutral stream events. The
generic pipeline converts between adapters and can run privacy/optional memory
recording. ModuAPI deliberately keeps bytes opaque instead of promising universal
chat translation. That preserves unknown input bytes on its common path, but
currently provides much less semantic interoperability. A protocol trait or a
small Rust gateway is not a market gap we discovered (G1–G3).

**LiteLLM covers the broad gateway/SDK use case.** Router has strategy/cooldown/
affinity/retry/fallback machinery; CustomLogger is a wide extension surface, not
just logging. The proxy composes auth, stores and hooks; local SDK/router use does
not inherently require Redis/Postgres. Current router source explicitly distinguishes
pre-output stream buffering and committed content when considering fallback.
ModuAPI must not claim exclusive ownership of partial-output safety (L1–L4).

## Anti-reinvention classification

| Class | Current mechanism | Assessment |
|---|---|---|
| A Commodity | independent processes, gRPC/protobuf, manifest, registry, health, drain/restart, cross-language transport | Existing process frameworks, Sub2API, APISIX and Envoy already solve variants. Maintaining custom bootstrap/supervision is a cost, not innovation. ADR 0002's dependency preference is not evidence we outperform go-plugin. |
| A Commodity | priority, weighted allocation, cooldown/EWMA, sticky, capacity reservations/expiring leases | Established selection/admission mechanisms. The local implementation is useful but no novel algorithm, distributed guarantee or exclusive capability is shown (N4,S3,A4,D1,L3). |
| B Useful but not unique | Protocol/Upstream/Scheduler interfaces, routing/selection/retry separation, opaque metadata | Good separation seen in adapters, schedulers, proxy filters and transport APIs. Explicit boundaries reduce local coupling; neither interfaces nor naming are a moat. |
| B Useful but not unique | frame/error/cancellation/lease ownership contract + Python conformance | Concrete engineering value and a reusable regression corpus. Protocol compatibility suites and stream ownership are established practices. Python fixtures prove this v1 path, not a thriving multi-language ecosystem. |
| C Potentially differentiated | protocol, transport and *whole* scheduler independently replaceable through one provider/account-free process contract | A relatively specific combination among these reviewed boundaries. Must still demonstrate two independent real protocol implementations, a replacement by an external author and an embedding use case. |
| C Potentially differentiated | a reusable composition kernel whose executable is only one assembly | Public packages/interfaces exist, but the convenient composition root is internal/app, the public Go API is not stabilized, and there is no external embedder test/adopter. This is a hypothesis, not a delivered product. |
| D Actually differentiated, bounded claim | the present artifact's **combination**: three replaceable process services + opaque body/candidates + no account/credential/billing host API + tested Go/Python ownership semantics | This can be checked in M1–M6 and differs from the inspected extension boundaries above. It is a difference in contract composition, **not proof of global uniqueness, demand or a durable competitive advantage**. No individual mechanism qualifies as new. |

The concern “Sub2API runtime + APISIX external plugins + llm-d scheduling + llmgate
adapters” is substantially correct as intellectual ancestry. We have independently
implemented established ideas. The unresolved question is whether packaging those
boundaries as a small reusable *composition kernel* saves an integrator enough work
to justify another runtime. Merely reimplementing each mechanism cleanly does not.

## The process-boundary bill

On a successful first attempt: **Decode → Select → Prepare → Execute → Transform
→ Report = 6 business RPCs**, four unary and two streaming. Protocol receives
three calls, scheduler two, upstream one. Lifecycle/health RPCs are extra, amortized
background work. Each additional attempt repeats selection/preparation/execution/
report and possibly transformation; it does not repeat ingress Decode (M1,M2).

Even with no model mapping, the entire request body crosses the wire six times:
Decode request/reply, Prepare request/reply, Execute input, Transform initial input.
A raw response data chunk takes upstream→host→protocol→host, i.e. three gRPC payload
legs before the downstream write. Chunk count, flushes, HTTP/2 windows and copying
matter; “six RPCs” is not six packets or six fixed-cost context switches.

The [end-to-end experiment](../benchmarks/end-to-end.md) compares actual processes,
JSON and incremental SSE against the same direct mock, including measured upstream
work and resource scope. The increment includes HTTP forwarding, parsing, guards,
timers and scheduling as well as IPC. Without an in-process ablation, attributing
all of it to gRPC or quoting a cost per RPC would be false precision. Scheduler
microbenchmarks cannot establish gateway speed. No competitors' marketing RPS
numbers are used as comparable measurements.

## What “lightweight” can and cannot mean

| Footprint | Evidence at baseline | Defensible claim |
|---|---|---|
| Code | core: 7 production Go files / 764 physical lines; runtime 7 / 1,287; SDK 5 / 492; official plugins 11 / 1,102; generated Go 2 / 2,473; internal composition/helpers 4 / 189; CLI/mock 2 / 151; build/conformance tools 2 / 73. go.mod has three direct and four indirect modules. Counts include comments/blanks, exclude `_test.go`; measured from tracked baseline files. | Small handwritten host surface; generated/transitive code still exists. Not an SLOC or dependency-risk ranking. |
| Runtime | one minimal kernel + three plugin processes, gRPC connections/health/supervision | Isolation has a measured cost; cannot promise smallest runtime. |
| Deployment | four executable artifacts/three manifests for minimal slice; native plugin binaries in this Windows build roughly 15.7–17.7 MiB each | No DB/Redis service requirement, but not a single-binary deployment. |
| Configuration | core only wires server/routes/plugins/upstreams; plugin objects are opaque | Small kernel schema; operator still understands several component configurations. |
| Operations | per-plugin readiness/startup/version/config/interpreter/crash state; all discovered plugins required | Fewer business services; more component failure modes than an in-process proxy. |
| Latency | real gateway increment in linked measurements | No unqualified low-latency/high-performance claim. A few ms may matter for local inference even if hidden by a slow WAN model. |
| Memory | sum of OS resident working sets across kernel + plugins, not just host heap | No unqualified low-memory claim. Shared pages can be double-counted by RSS sums; no PSS measurement on Windows. |
| Features | no user/account/billing/storage/admin domain required | **Small mandatory domain surface** is the currently substantiated meaning. Absence of features alone is not unique. |

Important packaging discrepancy: tools/build builds **five** official packages;
internal/app starts **every discovered package**, even if no route uses it.
The stock directory therefore starts kernel + five plugins = **six service
processes**. The benchmark deliberately selects exactly three to measure the
requested minimal topology. This default discovery/readiness behavior is debt,
not an optimization quietly applied to production for the benchmark (M2,M8).

## Are the Plugin/Host boundaries right?

Keep protocol interpretation and selection policy outside the host. Keep HTTP
commitment, replay prohibition after commitment, deadlines/cancellation and stream
ownership inside it. Keep upstream connection pools with the executor that owns
them. These boundaries remain useful. But “the host knows nothing” overstates reality:
it requires a model, owns token-shaped usage summaries, HTTP method/path/status,
fixed stage order, plugin service roles and a flat list of capability tokens (M1,M3).
It is an **AI HTTP data-plane kernel**, not a universal protocol runtime.

Retain the three **typed v1 services**, including their existing errors and stream
semantics. They allow replacement at distinct ownership points. Do not equate a
service with a package or process: the manifest, SDK and Python fixture already
allow one process to provide all three services. Official packaging currently uses
three processes. Collocation or a future execution profile needs measurement;
typed component separation does not require paying every possible process boundary.

Freeze expansion of the kind list. sdk/extensions.go already reserves many kinds
without wire services, and registry currently accepts only three. That is a warning
sign, not a platform roadmap. Describe `provides` as implemented, versioned service
capabilities; in a later ADR consider discovery descriptors for optional extension
points. Any hook must declare phase, ordering, input mutability, deadline and failure
policy. Do **not** replace typed RPC with `Call(kind, bytes)` or an arbitrary pipeline
DSL: that would move all semantic complexity into unversioned payloads. llm-d's
internal Filter/Score/Pick belongs inside a scheduler implementation unless an actual
independent sub-policy integration requires a host-level contract.

### Host capabilities: design need, not immediate implementation

There is no Plugin→Host service today. That is adequate for the current pure protocol,
local scheduler and direct HTTP executor. It becomes limiting for coordinated
diagnostics/context and operator-managed resources. Sub2API's optional broker
callback and APISIX ExtraInfo show this is an established pattern (S1,A1).

A future minimal control API should first justify scoped, bounded diagnostic events
and request/attempt correlation, with independent service version/capability discovery,
UNIMPLEMENTED fallback, deadline/cancellation and reentrancy rules. Avoid synchronous
callback cycles while a host is waiting on the same plugin. Metrics should use a
bounded export path, not a blocking RPC per token. Config delivery already exists.
Networking/pools stay executor-owned. No generic account directory, credential
resolver, secret store or persistent KV is justified by this audit; adding them now
would enlarge the kernel and introduce precisely the business/resource lifecycle
coupling we are testing. A concrete future secret need requires its own threat model
and contract. **No Host API is added here.**

### Trust model

The supported model is **trusted, operator-installed extensions under the host's OS
user**. They can access that user's filesystem/network regardless of manifest grants.
Restricted environment inheritance, package-local resolution, loopback tokens and
bounded diagnostics prevent mistakes; they do not confine a malicious plugin.
Bundling multiple services expands the failure/trust domain accordingly.

Installing arbitrary downloaded code is explicitly outside this model. A future
untrusted marketplace would require a new security/distribution design, including
confinement and provenance, before execution could be promised safe. Process
isolation is fault containment, not a security sandbox. No sandbox, signing system
or marketplace implementation is recommended merely to complete a checklist.

## Options and falsification

| | Option A — Continue narrowly | Option B — Pivot to standalone plugin runtime | Option C — Stop |
|---|---|---|---|
| Product | Composable AI HTTP gateway **construction kernel**, trusted components | Generic/AI external component runtime, possibly behind existing proxy | Use existing gateway/SDK/processor |
| Target | engineers needing independent component release/language/fault boundaries while owning edge identity/control plane | teams embedding external components in multiple hosts | teams needing a working relay, business gateway, policy proxy or inference cluster now |
| Keep | safety kernel, wire contract, conformance, runtime/registry, plugins as reference assemblies | contract tests/process code, selected stream helpers | documented lessons, reproducible harness/test corpus |
| Change/remove | qualify claims; validate public embedding, second protocol and execution placement **before** features | remove standalone HTTP/route/retry product responsibilities, split host-neutral runtime; adapt to ext_proc/APISIX only if adopter needs it | archive as prototype; contribute generic regression tests/ideas upstream with permission/license review |
| Main risk | combination has no adopter; IPC/copy/ops bill exceeds saved work | more commodity overlap with go-plugin/ext_proc/APISIX; no unique runtime requirement shown | forego a potentially useful small integration contract |
| Evidence now | boundary separation and Go/Python behavior exist; demand and independent composition unproven | no evidence a standalone runtime is needed; most lifecycle is commodity | mature alternatives cover broad gateway needs today |
| Judgment | **conditional Go for bounded validation** | No-Go as default pivot; require a specific host integration | rational exit if validation fails; not rejected because of sunk cost |

Why choose A instead of Sub2API/New API? Only if account/business-free component
replacement is a hard requirement; otherwise use their mature platforms. Instead
of APISIX/Envoy? Only if a small embeddable composition contract and operator-owned
component processes save more integration work than configuring a mature proxy;
existing proxy deployments should normally extend that proxy first. Instead of
Bifrost/LiteLLM? Only if avoiding host-owned provider/AI DTO evolution and independent
non-Go component delivery matter more than broad built-in provider support and
in-process cost. Instead of llm-d? Not for Kubernetes cache-aware inference scheduling;
use its EPP. Instead of llmgate? Not for ready-made neutral chat translation/privacy.

The next stage is a **validation budget, not authorization to add features**:

1. Obtain two concrete integration specifications from prospective users: one
   external component author and one embedder. Compare the actual integration work
   with Bifrost library and an Envoy/APISIX extension. “Could use” is insufficient.
2. Reproduce E2E on a dedicated Linux host; add larger bodies/slow consumers and
   compare an existing multi-service bundle and a temporary in-process control.
   Inspect CPU/allocation profiles before proposing boundary changes.
3. Demonstrate a second independently authored protocol using existing services,
   preserving unknown payloads, cancellation and retry safety without kernel edits.
   This is a proposed future validation experiment, not implemented in this audit.
4. Demonstrate external Go embedding using public packages and only then specify
   the smallest public composition API. Keep lifecycle safety invariant across any
   execution placement. Do not commit to a generic pipeline or HostService expansion.

Proposed engineering gates (hypotheses to confirm with users): for a 1KiB local
payload at concurrency 16 on the agreed Linux host, gateway-added p95 <= 5ms and
minimum four-process idle RSS <= 128MiB; preserve zero contract regressions and
no monotonic goroutine/FD growth in a 30-minute cancellation/slow-consumer run.
Current Windows measurements do **not** establish these Linux gates. If users need
sub-ms overhead or a single process and component isolation provides no benefit,
recommend an existing in-process gateway rather than rationalize the cost. If no
independent integration need or acceptable measured execution profile emerges,
choose Stop; do not reopen the Auth/Billing/Admin backlog to manufacture demand.

The [roadmap](../roadmap.md) and ADR 0009 make this stop condition explicit. There
is a verifiable combination to test, but no defensible claim of a new algorithm,
new plugin mechanism, universal superiority or established product-market fit.
