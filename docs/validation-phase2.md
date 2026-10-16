# Phase 2 validation

Local validation: 2026-10-16, Windows/amd64, Go 1.27.2, Python 3.11.5,
CGO/GCC enabled. Python packages installed only in `.venv-conformance` at the
versions in the committed requirements file. No external model APIs were called.

| Command | Result |
|---|---|
| `go test ./...` | PASS; final integration package 63.976s |
| `go test -race ./...` | PASS; final integration package 126.793s; child Go plugins/CLI instrumented |
| `go vet ./...` | PASS |
| `make conformance` | PASS; race-enabled Go host ↔ real Python processes, 9.786s |
| `go run ./tools/build` | PASS; host + five official process bundles |
| Advanced manifest CLI validation | PASS |
| Scheduler benchmark matrix | PASS; 10/100/1000 candidates, 1/10/100/1000 workers |
| Documentation links / `git diff --check` | PASS |

The Python suite covers discovery, all four services and all RPCs, matching and
mismatched handshake identities/versions/capabilities, JSON object validation,
manually appended unknown wire fields in both directions, 10MiB message limits,
legal and illegal frame sequences (including empty data and post-terminal frames),
Failure details/frames and unstructured gRPC status fallback. Cancellation tests
observe HTTP completion, Python Execute/Transform exit events, source closure,
forwarding goroutine joins, bounded goroutine counts, connection shutdown and
process reaping. Shutdown rejects new work while existing calls drain; stdin EOF
exits normally. Ordinary Go checks never require Python.

Scheduler tests cover priority fallback/exclusions, seeded large-sample weights,
zero-weight semantics, EWMA/cooldown/recovery/neutral errors, capacity snapshots and
100/1000 concurrent admissions, parallel/double feedback, all release outcomes,
expiry cleanup, bounded stale state, cancelled Select before/after reservation,
cancelled Report, successful-only sticky binding, TTL/LRU/oversized hints,
off/prefer/strict modes and priority/weight/health/capacity/exclusion escapes.
Real gRPC scheduler processes cover selection, feedback, leases, health, sticky,
explicit restart and proxy rebinding. A real Gateway routes requests through the
advanced process, protocol process and HTTP upstream process to local mocks.

Supervision tests exercise unexpected crash recovery, protected crash loops,
healthz/readyz separation, intentional restart/stop, replacement startup failure,
backoff cancellation on Close and closed-vocabulary bounded stderr diagnostics.
Privacy tests use synthetic markers only. Existing JSON/SSE, timeout, cancellation,
commitment/replay, pooling and CLI integration tests continue to pass.

No protobuf fields, numbers, messages or RPCs changed; Go dependencies are unchanged.
See [full benchmark output and timing boundaries](benchmarks/README.md). This is a
local policy benchmark, not an end-to-end gateway throughput target. CI includes
Go tests and Python conformance on Windows and Linux; remote results are available
in the repository's Actions runs and are separate from this local report.

Windows CI exposed a test-only path assumption: EvalSymlinks may normalize casing
or expand an 8.3 temporary-directory alias. The command-boundary test now compares
file identity with os.SameFile; the runtime's package/symlink containment check is
unchanged. Local ordinary/race/vet checks were rerun after this correction.

Known limits remain explicit: conservative overlapping snapshot/reservation counts,
best-effort Report + TTL recovery, local state reset on restart, no OS sandbox,
restricted stderr event vocabulary, no live diagnostics CLI or required validation
RPC, and no user/auth/billing/database/admin systems.
