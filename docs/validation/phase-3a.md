# Phase 3A validation harness

Phase 3A tests the composition claim from the perspective of an integrator. It
does not add a product feature or a new service kind. The experiment keeps the
existing three typed services and changes only their execution placement.

## Composition evidence

`plugins/reference/protocol-anthropic` is a Python process that implements the
existing Protocol lifecycle and gRPC contract directly from generated protobuf
types. It accepts a small Anthropic Messages subset and forwards its original
JSON body. `tests/conformance/anthropic` starts it through the normal manifest,
handshake, configuration, health, gateway, HTTP upstream and shutdown path. The
Core package has no Anthropic branch and the protobuf file is unchanged.

The external-package test in [`embedding/embedding_test.go`](../../embedding/embedding_test.go)
imports `embedding`, `core/config` and `core/router`, starts a runtime, serves a
real request through an `httptest` listener and closes it. It is written as
package `embedding_test`, so it cannot use unexported identifiers from the
embedding package. The build/mock helpers are test fixtures; they are not part
of the proposed consumer API.

The measured assembly work is small but not zero:

| Integration task | Measured artifact | LOC | What it proves |
|---|---|---:|---|
| Embed a reference gateway | external Go test | 52 | listener ownership and `ServeHTTP` composition are sufficient for a real request |
| Add a second protocol | Python process | 292 | protocol parsing, stream validation and usage semantics can live outside Core |
| Exercise the process contract | Go conformance test | 123 | independent protocol can use existing scheduler/upstream services |
| In-process placement control | audit-only adapter | 105 | local execution can be compared without changing production Registry/SPI |

The 292 lines include protocol-specific validation and SSE interpretation; the
generated protobuf bindings and dependency installation are separate. These are
not claims that the same work would be cheaper than every existing gateway.
There is no independent adopter baseline yet, so “saved LOC” is not treated as
an observed fact. The harness can state the remaining work precisely:

* the host still owns listener/TLS, configuration, deployment, authentication
  and secrets, admission policy and application-level tests;
* the protocol author owns its wire schema, response events, usage extraction
  and protocol-specific error rules;
* the upstream author owns HTTP transport, pooling and credentials granted by
  its manifest;
* the scheduler author owns policy state, health, affinity and leases;
* the Kernel owns cancellation, deadlines, replay safety, route candidate
  construction, stream ownership and commitment semantics.

Adding another component requires a manifest, a versioned service implementation
and configuration. It does not require a Core switch, a provider DTO, or a new
plugin kind. The cost remains meaningful: a component author must understand the
wire contract, lifecycle, frame terminal rules and failure attribution.

## Placement experiment

The Linux experiment was cross-compiled with Go 1.27.2 and `CGO_ENABLED=0`,
then run in Docker Desktop WSL2 (kernel `6.18.33.2-microsoft-standard-WSL2`)
from the mounted workspace. The harness starts a fresh process group for every
case and records raw per-case distributions in:

* [`phase-3a-linux-performance.json`](phase-3a-linux-performance.json): 64
  cases, two repetitions, 1 and 16 workers, 1KiB/64KiB/1MiB JSON and SSE;
* [`phase-3a-linux-stress.json`](phase-3a-linux-stress.json): 10-second slow
  consumers, continuous cancellation, and a plugin crash/restart case;
* [`phase-3a-linux-soak.json`](phase-3a-linux-soak.json): 30-second mixed
  stability workload.

The four placements are `direct` (mock only), `split` (Kernel plus Protocol,
Scheduler and Upstream processes), `bundle` (Kernel plus one process providing
all three typed services), and `local` (the audit-only in-process control).
The direct row is a reference, not an equivalent gateway. `local` is a control
for host execution cost; it is not a proposed production implementation.

An ordinary successful request crosses four unary and two streaming business
RPCs in the split/bundle process paths. A response data frame is serialized on
the upstream-to-host, host-to-protocol and protocol-to-host legs. The measured
residual is the complete gateway path after subtracting the mock's measured work;
it is not labelled “gRPC cost”. It includes HTTP, scheduling, copies,
serialization, process scheduling, queueing and runtime bookkeeping.

The three controls provide bounded attribution rather than a false single
number. `direct ? local` is the Kernel and in-process component cost;
`local ? bundle` is the additional bundle process, RPC and serialization cost;
`bundle ? split` is the incremental cost of separating the three typed
services, including extra process management. These are differences between
complete topologies, so they still are not causal microbenchmarks of one gRPC
call or one copy.

For c16 1KiB JSON, the median p95 comparisons are approximately 2.4ms direct,
4.0ms local, 7.2ms bundle and 7.3ms split. That bounds the local Kernel portion
near 1.6ms and the external bundle portion near 3.2ms for this workload; the
extra split process separation is below the run's rounding noise. At 1MiB,
the corresponding values are about 13ms, 31ms, 114ms and 110ms, showing that
serialization/buffering dominates and that process attribution is unstable at
large payloads. CPU, RSS and FD columns expose the separate multi-process
management cost directly.

At concurrency 16, medians across the two repetitions were:

| Workload | Placement | p50 / p95 / p99 total (ms) | RPS | Peak RSS | Peak FDs |
|---|---|---:|---:|---:|---:|
| 1KiB JSON | split | 5.65 / 7.32 / 8.28 | 2,788 | 78 MiB | 75 |
| 1KiB JSON | bundle | 5.55 / 7.19 / 8.12 | 2,848 | 43 MiB | 51 |
| 1KiB JSON | local | 2.38 / 4.04 / 4.97 | 6,253 | 24 MiB | 39 |
| 64KiB JSON | split | 12.20 / 15.51 / 17.14 | 1,296 | 178 MiB | 74 |
| 64KiB JSON | bundle | 12.12 / 15.26 / 16.64 | 1,307 | 133 MiB | 51 |
| 64KiB JSON | local | 4.51 / 7.10 / 8.51 | 3,399 | 31 MiB | 39 |
| 1MiB JSON | split | 88.66 / 110.10 / 117.22 | 178 | 247 MiB | 68 |
| 1MiB JSON | bundle | 96.21 / 113.75 / 121.18 | 165 | 203 MiB | 47 |
| 1MiB JSON | local | 23.64 / 30.86 / 34.17 | 676 | 152 MiB | 39 |
| SSE | split | 9.33 / 11.24 / 12.19 | 1,695 | 76 MiB | 75 |
| SSE | bundle | 9.34 / 11.17 / 11.96 | 1,697 | 44 MiB | 51 |
| SSE | local | 6.58 / 8.15 / 8.93 | 2,394 | 24 MiB | 39 |

The latency and RPS columns use medians of the two repetitions; RSS and FD are
the largest sampled value across those repetitions. The largest single observed
gateway RSS was 248MiB for split 1MiB JSON. Bundle
placement cuts the small-body process count from four to two and roughly halves
RSS/FDs, but it does not remove the serialization and process boundary. Large
bodies dominate every placement; the local control is also expensive because
the current Kernel buffers and validates the payload.

The stress run completed with zero errors. Cancellation counts matched completed
cancel requests; host goroutines and FDs returned to their post-case baselines.
Killing a ready Upstream process recovered in 266ms in split and 233ms in bundle,
with no successful-request errors after recovery. The mixed 30-second run showed
no monotonic growth in host goroutines or FDs, but it is still a short
workstation/WSL run, not a leak proof or SLA.

Repeat the experiment with:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o dist/audit-linux/phase3a ./tools/validate-phase3a
./dist/audit-linux/phase3a -build=false -bundles dist/audit-linux/plugins \
  -concurrency 1,16 -duration 3s -warmup 1s -repeats 2 \
  -topologies direct,split,bundle,local \
  -scenarios json1k,json64k,json1m,sse \
  -out docs/validation/phase-3a-linux-performance.json
```

The placement result supports bundle processes as a useful deployment option;
it does not support calling the split topology lightweight or claiming an
isolated RPC surcharge from the residual alone.
