# Current limits and next milestones

Phase 2 adds a tested language-neutral contract, independent Python reference,
generic advanced scheduler, bounded process supervision and private diagnostics.
Phase 1's kernel/runtime/SPI boundaries remain intact. No business system was added.

## Current technical debt

| Area | Remaining limit |
|---|---|
| SPI | Protocol/upstream/scheduler only; future kinds remain names/sketches |
| Validation | No additive ValidateConfiguration RPC/full JSON Schema; per-upstream config validated on use |
| Admission | Snapshot + reservations deliberately double-count active overlap; no begin/ack RPC |
| Leases | Best-effort Report; TTL must exceed maximum request lifetime; lost responses recover by expiry |
| State | Health/sticky/reservations local to scheduler process; reset on restart; no durability/distribution |
| Performance | Linear parsing/scanning and one scheduler mutex; allocations grow with candidate count; IPC costs separate |
| Recovery | Startup failure stops automatic recovery; fixed daemon defaults, embedder-configurable options; no operator control RPC/CLI |
| Readiness | All installed bundles required; no optional-plugin dependency graph |
| Diagnostics | Closed-vocabulary stderr tail only; no arbitrary traceback/body collection or event delivery guarantees |
| Security | Trusted processes/interpreters under host user; cooperative grants, no OS sandbox/signatures |
| Protocol | JSON/SSE only; no WebSocket, resumption or lossless cross-protocol translation |
| Bodies | Bounded ingress buffering, 4MiB JSON observation, 1MiB SSE event/line; no spooling |
| Usage | Best effort; immediate disconnect cancellation, no durable events/billing |
| Platforms | Go↔Go and Go↔Python verified locally on Windows; Linux/Windows CI configured, no Rust/Node reference yet |
| Release | Owner must choose project LICENSE; no signed packages, installer, SBOM or public release process |

## Next stage

1. Review the measured contract/admission limits. Design an **optional additive**
   configuration validation RPC with UNIMPLEMENTED fallback and a lease activation
   acknowledgement if measured capacity utilization justifies it. Preserve v1 peers.
2. Specify independent Auth/Secret/Middleware/Usage wire contracts, with one narrow
   reference implementation at a time. Credential acquisition and identity policy
   remain outside the kernel; no users, billing, quota, database or admin dependency.
3. Extend end-to-end benchmarks to JSON/SSE latency, backpressure, cancellation
   storms, file descriptors and pool reuse. Existing scheduler benchmarks measure
   local policy, not gateway throughput. Optimize only demonstrated bottlenecks.
4. Add Rust/Node conformance against the same wire semantics before packaging or a
   marketplace. Threat-model OS sandboxing, interpreter pinning, signatures and
   scoped host services separately. Do not advertise process isolation as a sandbox.

Distribution/Admin remain later phases. PostgreSQL, Redis, account pools, OAuth,
quota and billing are neither implemented nor prerequisites for a functioning kernel.
