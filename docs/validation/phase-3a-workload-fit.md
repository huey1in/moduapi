# Workload fit and the unresolved native Linux cost gate

Date: 2026-10-17. No performance run was added by this documentation-only
closeout. Source: [Phase 3A method and raw data](phase-3a.md), collected on
Docker Desktop **WSL2**, not a dedicated native Linux server. Native dedicated
Linux validation: **BLOCKED**; this session has only the Windows workstation
and its `docker-desktop` WSL distribution, with no supplied dedicated host.
Shared GitHub runners also would not discharge that condition.

## Observations, not universal bounds

The following are medians of the recorded two c16 repetitions. Payload and
response behavior are those of the existing mock experiment. The local control
is audit code, not a supported production placement API.

| Workload | Local p95 | Bundle p95 | Split p95 | Difference from local (bundle / split) |
|---|---:|---:|---:|---:|
| 1KiB JSON | 4.04ms | 7.19ms | 7.32ms | +3.15 / +3.28ms; 1.78 / 1.81 times local total |
| 64KiB JSON | 7.10ms | 15.26ms | 15.51ms | +8.16 / +8.41ms |
| 1MiB JSON | 30.86ms | 113.75ms | 110.10ms | +82.89 / +79.24ms; 3.69 / 3.57 times local total |
| SSE | 8.15ms | 11.17ms | 11.24ms | +3.02 / +3.09ms total completion time |

These are differences of summary quantiles between complete topologies, not a
per-request subtraction or causal estimate of a single subsystem. Small JSON
RPS is 6,253 local / 2,848 bundle / 2,788 split; 1MiB is 676 / 165 / 178.
At small JSON the observed RSS is about 24 / 43 / 78MiB and FDs 39 / 51 / 75.
Bundle halves gateway/component processes (4 to 2) and saves memory/FDs, but
has little observed latency advantage over split. It does **not** repair the
large-body cost. Process counts exclude the common mock and load generator.

An ordinary successful external request makes four unary and two streaming
business RPCs. Request/response serialization, copies, HTTP processing,
scheduling, process scheduling and runtime management interact. Neither the
gateway residual nor local-to-external difference isolates gRPC, serialization
or supervision. Those individual costs remain **UNKNOWN** without controlled
profiling/ablations. SSE total latency is not TTFB; first-byte distributions
are separately recorded in the raw files and must be checked against the
participant's TTFB SLO. Do not substitute completion latency for first token.

## Suitability hypotheses to validate with the participant

| Load / constraint | Assessment from existing evidence | What still needs confirmation |
|---|---|---|
| Moderate-size requests to remote inference with a several-ms gateway budget; teams require foreign-language/release/fault boundaries | **Potential fit.** Small-body bundle data makes this plausible. Remote model latency does not erase the added CPU/RSS or tail latency. | Actual body distribution, QPS, TTFB SLO, acceptable CPU/memory per instance, requirement for process separation. |
| Long SSE generations | **Conditional.** Small-message completion results and cancellation tests support further validation, not a blanket claim. Concurrent streams occupy resources for longer. | Long-stream/slow-reader workload, chunks/sec, TTFB/tail gaps, connection/FD budget and 30-minute profile on dedicated Linux. |
| Same-language trusted hooks, existing Bifrost/APISIX/Envoy deployment | **Prefer the existing solution** unless an unmet boundary is shown. | Demonstrated maintenance savings large enough to justify another wire/runtime contract. |
| Local low-latency inference, short responses, high-QPS service or sub-ms gateway budget | **Poor fit for the current external path.** Observed multi-ms difference and throughput reduction are material. | A new production-shaped measurement could change the finding; no claim that AI compute always dominates. |
| 1MiB-class JSON or large prompt/context workloads | **Poor fit for a tight latency budget.** About 79–83ms extra p95 against local here, with lower throughput and higher RSS. | Participant response/body sizes and acceptable budget; do not hide this behind the 1KiB gate or start an unrequested rewrite. |
| Large uploads, binary/multimodal payloads or resumable sessions | **Not validated / outside current scope.** Current buffered caps and JSON/SSE contracts constrain it. | A different product requirement, not evidence to pass Phase 3A. |

## What the environment blocker means

The original <=8ms JSON / <=12ms SSE c16 engineering observations remain WSL
observations. No dedicated-machine upper bound, machine-to-machine portability
or production SLA has been established. GO still requires the original native
Linux repeat within 20%, original workload bounds, zero correctness errors and
a long resource profile. The roadmap's 30-minute stability target is also
outstanding; the existing soak was only 30 seconds.

When a host is available, record machine ownership/isolation, CPU/RAM/NUMA,
OS/kernel, Go version, power/governor settings, competing load, affinity and
command lines. Use the [existing reproduction commands](phase-3a.md); build
Linux binaries on that machine, preserve fresh raw files under new names, and
run the same direct/local/bundle/split workloads twice. Collect process-tree
CPU/RSS/FDs and host/child profiles with measurement scope declared. Never
overwrite WSL records or count a passing shared-CI run as dedicated-host
evidence. No new benchmark harness or production optimization is authorized by
this closeout.
