# Phase 3A exit gate

This gate decides whether the **composition claim** deserves one small next
step. It does not approve Auth, Secret, Middleware, Usage, storage, UI, a new
plugin kind, or a general pipeline language.

The gate is evaluated against the Linux raw records in
[`phase-3a.md`](phase-3a.md). A “pass” means the stated observable condition is
met; architecture preference and code cleanliness do not count.

## Conditions

| Area | GO condition | Observed result |
|---|---|---|
| Composition | A second wire protocol runs as a normal process plugin with no Core or protobuf edit; Protocol, Upstream and Scheduler can be changed by manifest/configuration. | **Pass.** Anthropic Messages Python process passes JSON and SSE through the existing gateway and generic upstream/scheduler. |
| Embedding | An external Go package can assemble the Kernel without importing `internal/app` or running the reference binary, serve a real request and close cleanly. | **Pass, experimental.** `package embedding_test` does this; the public API is explicitly not stable. |
| Placement cost | On a reproducible Linux run, c16 1KiB JSON p95 is <=8ms and SSE p95 is <=12ms for split and bundle, with zero errors; the result must show residual and direct reference separately. | **Pass for this workload.** Split/bundle p95 is about 7.2ms/11.2ms (JSON/SSE), with residual about 5.7ms/5.5ms. The cost is the whole path, not an attributed gRPC number. |
| Bundle value | Bundle has materially lower process/RSS/FD overhead than split while retaining the same typed contract. | **Pass.** At c16 1KiB JSON, bundle uses 2 vs 4 processes, about 43 vs 78MiB RSS and 51 vs 75 FDs. |
| Large payload behavior | 64KiB and 1MiB JSON, streaming and slow-consumer runs are recorded; no claim of a universal latency budget is allowed. | **Pass as evidence, fail as a low-latency claim.** 1MiB c16 p95 is about 110–114ms external and 31ms local. |
| Ownership and recovery | Cancellation, slow consumers, plugin crash/restart and a 30s mixed run complete with zero errors, no post-case resource growth, and bounded restart. | **Pass for the experiment.** Cancellation ownership is observed; recovery is 266ms split/233ms bundle; goroutine/FD baselines return. |
| Integration value | Two concrete independent integration specifications show less work or a required boundary unavailable in an existing solution. | **Not yet demonstrated.** The repository has repeatable synthetic harnesses and measured API work, but no independent adopter or competitor implementation baseline. |

## Decision rules

### GO

All rows above pass **and** two independent integrators provide written
requirements, one component author and one embedding host, with measured work
saved against the existing solution they would otherwise use. A native Linux
repeat must reproduce the placement bounds within 20% and include at least one
long-run resource profile. GO would authorize only the smallest placement/API
stabilization work described by those requirements.

### CONDITIONAL GO

The composition, embedding, placement, ownership and bundle rows pass, but
adoption evidence or dedicated-host reproducibility is still missing. Work may
continue only to obtain that evidence and to stabilize the experimental
embedding/placement measurements. No product plugin kinds or business features
may be added.

### NO-GO

Choose NO-GO if any of these occurs:

* the second protocol requires a Core/protobuf semantic branch, or a component
  cannot be independently replaced;
* an external host must import internal composition code or run the reference
  binary;
* split/bundle c16 1KiB JSON p95 exceeds 8ms or SSE p95 exceeds 12ms on the
  agreed Linux host in two clean repetitions, or error/cancellation/stream
  ownership fails;
* bundle placement does not materially reduce process/resource cost;
* large-body and streaming behavior cannot be measured without hidden errors or
  unbounded resource growth.

NO-GO means retain the research and regression tests, then choose an existing
  gateway/runtime for the relevant use case. It does not justify adding
  features to rescue the design.

### STOP

Choose STOP if the gate technically passes but, after the evidence window,
neither an independent component author nor an embedding host has a requirement
that needs replaceable cross-language components. A working abstraction without
that requirement is not sufficient reason to maintain another gateway kernel.

## Current verdict: CONDITIONAL GO

The second protocol, external embedding test, bundle placement, cancellation,
crash/restart and resource experiments pass their engineering checks. The Linux
results also show why “lightweight” must remain limited to the absence of
mandatory business services: split processes have substantial RSS/FD/CPU costs,
and large bodies amplify serialization and buffering. The project has not yet
shown a real integration that saves work versus Bifrost, APISIX/ext_proc,
llm-d, llmgate, LiteLLM, New API or Sub2API. Therefore the evidence supports a
bounded validation continuation, not a product expansion or a claim of
competitive novelty.

The next gate work is limited to obtaining the two integration specifications,
repeating the benchmark on a dedicated Linux host with profiles, and deciding
whether the experimental embedding API should be narrowed or discarded. If
those requirements do not arrive, the correct result is STOP.
