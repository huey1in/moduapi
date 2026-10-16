# Architecture audit validation

Date: 2026-10-16. Audited production source: `a3f011d405785886a5c723979d28b9ec78f61739`.
The working tree adds research, documentation and a measurement tool. There is no
production change under core/, runtime/, sdk/, proto/, gen/, plugins/ or tests/;
no protobuf/service/version change, dependency change or new plugin kind.

## Checks actually run

Windows 11 Home China build 26200, amd64, Go 1.27.2; race uses the w64devkit C
compiler. These are observed local wall times, not performance claims.

| Command | Result | Wall time |
|---|---|---:|
| `go test -count=1 ./...` | PASS, including actual process gateway/scheduler/runtime integration | 67.141s |
| `go test -race -count=1 ./...` | PASS, no reported race | 131.125s |
| `go vet ./...` | PASS | 1.922s |
| `make conformance` | PASS, pinned private Python environment and real Go/Python process RPC | 17.468s |

After making the harness output filename OS-specific, targeted
`go test -race ./tools/bench-e2e` and `go vet ./tools/bench-e2e` also passed.
That edit changes no measurement boundary or recorded numbers. Linux/amd64 harness
and all five official plugins were cross-compiled successfully with CGO_ENABLED=0;
three required bundles were exercised in each applicable WSL scenario.

Conformance regenerates the Python bindings from the existing contract. There is
no resulting tracked binding diff. The normal Go suite still needs no Python.
The existing CI runs Go/race/vet on Windows and Ubuntu, plus separate conformance;
this document does not claim a new remote CI result before it exists.

## Measurement checks

See [E2E methods, tables and limitations](benchmarks/end-to-end.md).

- Windows: 72 cases, three repetitions, four topologies, JSON/SSE, concurrency
  1/16/64; 1,193,793 successful measured requests, zero errors.
- WSL Linux: eight exploratory cases, one repetition at concurrency 16;
  129,773 successful measured requests, zero errors; real POSIX FD collection.
- Every gateway row has four service processes; every direct row has one target
  mock process. Gateway scopes exclude mock/load generator. Stock deployment's
  six-process discovery behavior was retained and explicitly distinguished.
- SSE first-body-byte p50 precedes completion p50 in every case. Harness tests
  separately verify timing payloads, incremental mock delivery, process resource
  sampling and monotonic measurement clocks.
- After client-idle cleanup, each gateway reports 31 **host** goroutines, direct
  targets 4. Helper shutdown waits/reaps processes. A post-run Windows process
  inspection and WSL process inspection found no remaining benchmark helpers or
  temporary benchmark plugin processes.
- Raw result files preserve every per-case statistic; tables use median-of-three
  Windows values and explicitly single-run WSL values. The initial coarse-clock
  run was discarded and is not committed as measurement evidence.

There is no claim of a 30-minute leak test, per-plugin goroutine instrumentation,
native dedicated Linux performance validation, in-process ablation or a competing
gateway benchmark. The Linux FD supplement does not turn Windows handles into FDs.
The harness's `Baseline` label identifies this audit snapshot; when using it after
production changes, record the actual tested revision alongside the new result.

## Document and scope checks

The comparison contains 30 dimensions x 9 project columns, each cell with a
relative label and pinned-source evidence references. Local links in changed/new
documents resolve. `gofmt -l tools/bench-e2e` and `git diff --check` are clean.
Existing ADRs are preserved; ADR 0009 supersedes feature-expansion assumptions.
The only build command addition is the optional `make bench-e2e` harness target.

## Remaining debt

The audit adds platform resource collectors and an OS-sensitive benchmark that
must be maintained. Windows RSS/handles and Linux RSS/FD differ in accounting;
short desktop measurements and sampled peaks are insufficient release gates.
There is no independent adopter, second real protocol or stable public composition
API yet. Existing kernel/runtime/scheduler limits remain in the
[revised roadmap](roadmap.md); none were hidden behind a benchmark-only optimization.
