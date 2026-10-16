# Roadmap: validate the reason to exist

Decision basis: [ADR 0009](adr/0009-moduapi-positioning.md),
[source audit](research/competitive-architecture-audit.md),
[E2E experiment](benchmarks/end-to-end.md). Baseline `a3f011d`.

## Completed foundations

Phase 1 established the small HTTP kernel, process runtime, three typed services,
official OpenAI-compatible JSON/SSE assembly and mock/integration tests. Phase 2
added Go/Python conformance, interpreter packages, generic local scheduling,
reservations/sticky/health and bounded supervision. These are useful foundations,
not evidence of novelty or demand.

## Phase 3A — Architecture / Positioning Validation

**Audit delivered:** eight projects inspected at pinned commits, thirty-dimension
comparison, commodity/differentiation classification, trust and boundary review,
real four-topology E2E harness/measurements, Continue/Pivot/Stop decision.
Conditional Go is for the validation below, not a broad feature roadmap.

**Next experiments, not implemented by the audit:**

1. Obtain two concrete integration specifications: an independent component author
   and an embedder. Compare effort with a Bifrost library assembly and APISIX/Envoy
   extension. Confirm independent language/release/fault boundaries are required.
2. Repeat E2E on a dedicated Linux host, including 1KiB/64KiB/1MiB requests, long
   SSE, slow consumers and cancellation storms. Record per-process CPU/RSS/FD,
   heap profiles, goroutines and copy/IPC costs. Compare the existing multi-service
   bundle contract and a temporary in-process control before choosing placement.
3. Demonstrate a second independent protocol using existing services without kernel
   edits. Require unknown-payload fidelity and existing stream/retry/cancellation
   safety. Avoid implementing a universal provider DTO.
4. Demonstrate an external Go module embedding public packages. Specify only the
   composition API actually needed; internal/app is not currently public.
5. Revisit capability discovery and a minimal optional Host diagnostic API only if
   these experiments establish a missing contract. Keep typed APIs; no arbitrary
   pipeline DSL or generic business-host-services surface.

Proposed acceptance hypotheses, to confirm against prospective workloads:

- At concurrency 16 / 1KiB bodies on an agreed Linux machine: gateway-added p95
  <= 5ms and minimal four-process idle RSS <= 128MiB. Windows results do not prove
  these gates. If users need sub-ms overhead, compare an existing in-process gateway.
- Zero ownership/commitment/cancellation conformance regressions; no monotonic
  goroutine/FD growth over a 30-minute slow-consumer/cancellation run.
- Both integration specifications demonstrate saved work or a required boundary
  unavailable through the selected existing solution. No unmet need means Stop.

## Phase 3B — determined by 3A evidence

No preassigned plugin backlog. If A passes, stabilize the smallest public composition
contract and select placement using measurements. If an adopter needs only a runtime
adapter, evaluate that narrow pivot with its own ADR. If integration/cost gates fail,
archive the prototype and use/contribute to the appropriate mature project. Code
volume is not an acceptance criterion.

**Auth, Secret, Middleware, Usage, cache, rate limiting and all new plugin kinds
remain paused.** Billing, quota, accounts, OAuth, DB, Redis, admin and marketplace
are outside this stage. No sandbox/signing project is started for trusted extensions
merely to make the runtime look complete.

## Current technical debt

| Area | Remaining limit |
|---|---|
| Positioning | Specific combination exists; independent demand/embedding/protocol interchange unproven |
| Public API | Three tested wire services; public Go composition API not stable; future kind sketches are not commitments |
| Placement | Six business RPCs; six request-body wire legs and three response-data legs; no in-process ablation |
| Packaging/readiness | All discovered bundles start and are required, even unused ones; stock directory runs six processes |
| Validation | No additive validation RPC/full JSON Schema; per-upstream options parsed on use |
| Admission | Snapshot + reservations double-count overlap; no begin/ack; lease TTL must exceed request lifetime |
| State | Local health/sticky/reservations reset on restart; Report best effort; no durable/distributed guarantee |
| Performance | Linear hint parsing/selection + mutex; Windows workstation plus exploratory WSL; no dedicated native Linux or competitor head-to-head benchmark |
| Measurement | Windows FD unavailable; handles separate; only host goroutines observable; Linux FDs exercised in single-run WSL supplement |
| Recovery | Replacement startup failure stops recovery; no online operator API/hot reload/process-tree containment |
| Diagnostics | Closed-vocabulary bounded stderr only; arbitrary tracebacks intentionally dropped |
| Security | Trusted same-user processes/interpreters; cooperative permissions, no hostile-plugin confinement |
| Protocol/body | One real chat protocol; buffered capped ingress, JSON/SSE limits; no uploads/spooling/WebSocket/resumption |
| Usage | Best-effort token summary; disconnect cancels upstream; no durable accounting |
| Release | Owner-selected LICENSE missing; no independent adopters/installer/SBOM/release compatibility matrix |

The audit adds evidence and a measurement tool. It does not resolve these limits
through feature expansion or a hidden production refactor.
