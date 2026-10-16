# Roadmap: validate the reason to exist

Decision basis: [ADR 0009](adr/0009-moduapi-positioning.md),
[source audit](research/competitive-architecture-audit.md),
[E2E experiment](benchmarks/end-to-end.md). Baseline `71d69e6`; Phase 3A raw evidence is in `docs/validation/`.

## Completed foundations

Phase 1 established the small HTTP kernel, process runtime, three typed services,
official OpenAI-compatible JSON/SSE assembly and mock/integration tests. Phase 2
added Go/Python conformance, interpreter packages, generic local scheduling,
reservations/sticky/health and bounded supervision. These are useful foundations,
not evidence of novelty or demand.

Phase 3A added only validation artifacts: an independent Python Anthropic
Messages reference process, an experimental Go embedding package, and a Linux
split/bundle/in-process placement harness. It did not add a provider catalog,
new PluginKind, or business feature.

## Phase 3A — Architecture / Positioning Validation

**Audit delivered:** eight projects inspected at pinned commits, thirty-dimension
comparison, commodity/differentiation classification, trust and boundary review,
real four-topology E2E harness/measurements, independent protocol, embedding and
placement validation. The current [exit gate](validation/phase-3a-exit-gate.md)
is **CONDITIONAL GO**: the engineering checks pass, but independent adopter
evidence and a dedicated-host repeat are still required.

**Next experiments, not implemented by the audit:**

1. Obtain two concrete integration specifications: an independent component author
   and an embedder. Compare effort with a Bifrost library assembly and APISIX/Envoy
   extension. Confirm independent language/release/fault boundaries are required.
2. Repeat the placement experiment on a dedicated native Linux host with CPU/heap
   profiles and the same raw workloads. Keep split/bundle/local evidence separate.
3. Decide whether the experimental embedding surface is narrow enough to retain;
   do not call it stable without an external integration.
4. If either requirement is absent, choose STOP and preserve the conformance,
   benchmark and research artifacts without adding features.

Proposed acceptance hypotheses, to confirm against prospective workloads:

- The current engineering gate uses c16 / 1KiB p95 <=8ms JSON and <=12ms SSE
  for split and bundle, with residual and direct rows retained. Large bodies are
  evidence, not a low-latency promise. If users need sub-ms overhead, compare an
  existing in-process gateway.
- Zero ownership/commitment/cancellation conformance regressions; no monotonic
  goroutine/FD growth over a 30-minute slow-consumer/cancellation run.
- Both integration specifications demonstrate saved work or a required boundary
  unavailable through the selected existing solution. No unmet need means Stop.

## Phase 3B — determined by 3A evidence

No preassigned plugin backlog. If the gate reaches GO, stabilize only the smallest
public composition contract and select placement using measured requirements. If an
adopter needs only a runtime adapter, evaluate that narrow pivot with its own ADR.
If integration/cost gates fail, archive the prototype and use/contribute to the
appropriate mature project. Code volume is not an acceptance criterion.

**Auth, Secret, Middleware, Usage, cache, rate limiting and all new plugin kinds
remain paused.** Billing, quota, accounts, OAuth, DB, Redis, admin and marketplace
are outside this stage. No sandbox/signing project is started for trusted extensions
merely to make the runtime look complete.

## Current technical debt

| Area | Remaining limit |
|---|---|
| Positioning | Specific combination exists; independent demand/embedding/protocol interchange unproven |
| Public API | Three tested wire services; public Go composition API not stable; future kind sketches are not commitments |
| Placement | Six business RPCs; six request-body wire legs and three response-data legs; native Linux and profile repeat still outstanding |
| Packaging/readiness | All discovered bundles start and are required, even unused ones; stock directory runs six processes |
| Validation | No additive validation RPC/full JSON Schema; per-upstream options parsed on use |
| Admission | Snapshot + reservations double-count overlap; no begin/ack; lease TTL must exceed request lifetime |
| State | Local health/sticky/reservations reset on restart; Report best effort; no durable/distributed guarantee |
| Performance | Linear hint parsing/selection + mutex; WSL placement evidence is complete for this gate; no dedicated native Linux or competitor head-to-head benchmark |
| Measurement | Windows FD unavailable; handles separate; only host goroutines observable; Linux FDs exercised in two-repeat WSL Phase 3A records |
| Recovery | Replacement startup failure stops recovery; no online operator API/hot reload/process-tree containment |
| Diagnostics | Closed-vocabulary bounded stderr only; arbitrary tracebacks intentionally dropped |
| Security | Trusted same-user processes/interpreters; cooperative permissions, no hostile-plugin confinement |
| Protocol/body | Two reference protocols; buffered capped ingress, JSON/SSE limits; no uploads/spooling/WebSocket/resumption |
| Usage | Best-effort token summary; disconnect cancels upstream; no durable accounting |
| Release | Owner-selected LICENSE missing; no independent adopters/installer/SBOM/release compatibility matrix |

The audit adds evidence and a measurement tool. It does not resolve these limits
through feature expansion or a hidden production refactor.
