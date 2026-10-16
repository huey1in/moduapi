# Current limits and next milestones

The first slice implements the kernel boundary and real process-based JSON/SSE traffic. The following work is deliberately explicit rather than implied by SDK type names.

## Current technical debt

| Area | Limit today |
|---|---|
| SPI | Stable namespaced v1 for protocol/upstream/scheduler only; other kinds have names/experimental Go sketches, no runnable wire services |
| Runtime | Manual `Manager.Restart`, no automatic bounded backoff, operator restart CLI, installation/update/signature flow or hot reload |
| Security | Trusted processes under host OS user; cooperative network check and env grants; no OS sandbox or hostile-local-user protection |
| Diagnostics | Host emits safe generic failures; child stderr is discarded; no bounded structured plugin log/event stream |
| Scheduling | Single and local round robin; no priority-weighted/health/sticky/concurrency queue plugin yet; no distributed state |
| Leases | Report is best effort; future reservation plugins must expire/recover leases, including cancelled Select calls |
| Config | Root schema declarations are validated; full JSON Schema is not; per-upstream plugin config errors surface on use |
| Streaming | SSE LF/CRLF and raw HTTP bodies; no WebSocket, resumption or lossless cross-protocol conversion |
| Bodies | Ingress buffered at 4MiB by default (8MiB max); JSON response observation at 4MiB; 1MiB SSE line/event limit; no disk spooling |
| Usage | Best effort token observation only; no persistence, billing, full-drain-after-disconnect or delivery guarantees |
| Performance | No capacity claim or benchmark baseline yet; gRPC copies bodies, response transformation adds an IPC leg; measure before optimizing |
| Language/platforms | Go plugins tested locally on Windows; CI configured for Windows/Linux; Rust/Node/Python interoperability not yet exercised |
| Distribution | No project LICENSE chosen by the owner, public version release, installer, signing or SBOM yet |

## Recommended next stage

1. Build a small cross-language conformance suite and a Python or Rust reference plugin. Freeze tested stream ownership, error details, unknown-field compatibility, shutdown and negotiation behavior before expanding the SPI.
2. Add **one** scheduler plugin implementing priority tiers, within-tier weight and health feedback, with explicit eligibility, cancellation, bounded reservations and sticky escape tests. Keep its formulas and state outside Core. Add weighted distribution tests and fault injection.
3. Add runtime operator diagnostics and bounded restart/backoff with crash-loop protection; make drain/replacement behavior observable. Introduce a generic plugin config validation RPC so upstream/protocol options fail before traffic.
4. Design the authenticator process API and a minimal env-secret broker. Keep credentials, identity policy and user/API-key management outside the kernel. Do not expand into billing or UI to make the demo look complete.
5. Benchmark JSON and SSE with concurrency, slow readers, long streams, provider disconnects and cancellation storms. Track allocations, first-byte latency, goroutines, file descriptors and reuse. Use the evidence to choose request streaming/spooling, fewer IPC body copies or pooling changes.

OS containment, signatures and secret/storage permissions need an explicit threat model and platform implementation before installing untrusted plugins. PostgreSQL, Redis, admin, users, payments and billing remain optional future plugins; none is a prerequisite for the next slice.
