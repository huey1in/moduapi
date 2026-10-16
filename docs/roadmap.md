# Roadmap: validate the reason to exist

Decision basis: [ADR 0009](adr/0009-moduapi-positioning.md),
[source audit](research/competitive-architecture-audit.md),
[E2E experiment](benchmarks/end-to-end.md). Engineering baseline `71d69e6`;
demand closeout baseline `dfde6e6` (2026-10-17). Phase 3A raw evidence is in
`docs/validation/`.

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
evidence and a dedicated-host repeat are still required. The
[public-demand review](research/external-demand-validation.md) is complete:
ten traceable leads/counterexamples, **zero confirmed independent component
authors, zero confirmed independent embedding hosts**. All public leads remain
UNVERIFIED. No external contact was sent or authorized. The native dedicated
Linux repeat is **BLOCKED**, not substituted by WSL2 or shared CI.

**Only remaining work: evidence collection, no feature/API development.**

1. Obtain two concrete integration specifications: an independent component author
   and an embedder. Compare effort with a Bifrost library assembly and APISIX/Envoy
   extension using the [same-task comparison](validation/phase-3a-alternatives.md)
   and [cost record](validation/integration-cost-record.md). Confirm independent
   language/release/fault boundaries are required. [Component-author](validation/component-author-validation.md)
   and [embedding-host](validation/embedding-host-validation.md) briefs are ready;
   publication/contact still requires explicit authorization.
2. Repeat the placement experiment on a dedicated native Linux host with CPU/heap
   profiles and the same raw workloads. Keep split/bundle/local evidence separate.
3. Retain the experimental embedding Handler for validation. The
   [API review](validation/embedding-api-review.md) recommends narrowing its
   exposed configuration only against a qualifying external host; implement
   no change now. Current API/export count does not prove stability.
4. Proposed final evidence review: **2026-11-07**, at most five candidate
   projects, no automatic outreach or rolling extension. If neither cohort
   qualifies after the bounded validation, choose STOP and preserve research
   and regression artifacts. One qualifying cohort still fails GO and supports
   at most an explicitly evaluated narrow pivot, not feature development.

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
| Positioning | Engineering interchange/experimental embedding demonstrated; independent demand and net integration savings unproven; public leads all UNVERIFIED |
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
| Release | Owner-selected LICENSE missing (adoption/distribution blocker); no independent adopters/installer/SBOM/release compatibility matrix |

The audit adds evidence and a measurement tool. It does not resolve these limits
through feature expansion or a hidden production refactor.
