# Phase 3A exit gate

This gate decides whether the **composition claim** deserves one small next
step. It does not approve Auth, Secret, Middleware, Usage, storage, UI, a new
plugin kind, or a general pipeline language.

The gate is evaluated against the WSL2 Linux raw records in
[`phase-3a.md`](phase-3a.md), with the dedicated native Linux repeat still
required below. A “pass” means the stated observable condition is met;
architecture preference and code cleanliness do not count. Closeout reviewed
2026-10-17 at `dfde6e6`; the original numerical and independence conditions
are unchanged.

## Conditions

| Area | GO condition | Observed result |
|---|---|---|
| Composition | A second wire protocol runs as a normal process plugin with no Core or protobuf edit; Protocol, Upstream and Scheduler can be changed by manifest/configuration. | **Pass.** Anthropic Messages Python process passes JSON and SSE through the existing gateway and generic upstream/scheduler. |
| Embedding | An external Go package can assemble the Kernel without importing `internal/app` or running the reference binary, serve a real request and close cleanly. | **Pass, experimental.** Repository package test plus executed separate-module documentation example with real HTTP listeners and no internal imports; this is not an independent adopter. |
| Placement cost | On a reproducible Linux run, c16 1KiB JSON p95 is <=8ms and SSE p95 is <=12ms for split and bundle, with zero errors; the result must show residual and direct reference separately. | **Pass for this WSL2 workload only.** Split/bundle p95 is about 7.2ms/11.2ms (JSON/SSE), with residual about 5.7ms/5.5ms. The cost is the whole path, not an attributed gRPC number. Dedicated native Linux remains BLOCKED. |
| Bundle value | Bundle has materially lower process/RSS/FD overhead than split while retaining the same typed contract. | **Pass.** At c16 1KiB JSON, bundle uses 2 vs 4 processes, about 43 vs 78MiB RSS and 51 vs 75 FDs. |
| Large payload behavior | 64KiB and 1MiB JSON, streaming and slow-consumer runs are recorded; no claim of a universal latency budget is allowed. | **Pass as evidence, fail as a low-latency claim.** 1MiB c16 p95 is about 110–114ms external and 31ms local. |
| Ownership and recovery | Cancellation, slow consumers, plugin crash/restart and a 30s mixed run complete with zero errors, no post-case resource growth, and bounded restart. | **Pass for the experiment.** Cancellation ownership is observed; recovery is 266ms split/233ms bundle; goroutine/FD baselines return. |
| Integration value | Two concrete independent integration specifications show less work or a required boundary unavailable in an existing solution. | **Not demonstrated.** Public leads are all UNVERIFIED; confirmed independent component authors = 0, embedding hosts = 0. No measured alternative integration baseline or saved work. See the demand register below. |

## Closeout evidence and blockers

| Item | State at closeout | Consequence |
|---|---|---|
| Public-source demand review | Complete; ten leads/counterexamples with dates, authors, resolutions and alternatives in the [evidence register](../research/external-demand-validation.md). | Separate extension needs exist, but none confirms the combined ModuAPI requirement or adopter benefit. |
| Component-author and embedding-host confirmation | **BLOCKED**: no interviews/participant confirmations available; no outreach sent or authorized. [Two briefs](component-author-validation.md) and an [external-module host example](embedding-host-validation.md) are prepared. | Our examples and public discussions cannot pass the independent-integration row. |
| Same-scope alternative implementation | **UNKNOWN**: source-level [comparison](phase-3a-alternatives.md) complete; no participant-owned trials. | Saved LOC/hours and net maintenance savings remain unmeasured; use the [cost record](integration-cost-record.md). |
| Dedicated native Linux reproduction | **BLOCKED**: only Windows/WSL2 is accessible; no dedicated native host supplied. | WSL2 or shared CI cannot satisfy the original native repeat/20% requirement. |
| Long resource profile | **OUTSTANDING**: existing mixed run = 30 seconds; roadmap target = 30 minutes. | Short stability observations are not a production leak/SLA proof. |
| Embedding API stability | **Experimental**; [retain/narrow/delete review](embedding-api-review.md) completed, no API changes. | Four methods still expose configuration/router types and leave lifecycle/operations work to the host. |

The [workload assessment](phase-3a-workload-fit.md) retains the large-body
counterevidence: 1MiB JSON adds about 79–83ms p95 against the local control in
these runs. Small-body success does not approve large-body or sub-ms workloads.

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

The next gate work is limited to obtaining the two integration specifications
and participant-reviewed benefits, and repeating the benchmark on a dedicated
Linux host with profiles. The API review recommends retaining the experimental
Handler seam and considering a narrower configuration surface only if a real
host qualifies. **No code/API stabilization or feature expansion is authorized
by this documentation closeout.**

The public-source pass is complete. Proposed final evidence review is
**2026-11-07**, with a maximum of five candidate projects and no automatic
outreach or rolling extension; see the [bounded evidence plan](../research/external-demand-validation.md#bounded-next-decision).
No qualified demand after that bounded validation means STOP under the rule
above. One qualifying cohort still cannot pass GO. Today there is insufficient
evidence either for product GO or for claiming an exhaustive negative market
result; CONDITIONAL GO authorizes only evidence collection.

## Closeout verification

On Windows/Go 1.27.2: `go test ./...`, `go test -race ./...` (existing package
results reused Go's valid cache), `go vet ./...`, and the fresh race-enabled
`go run ./tools/conformance` all passed. The standalone host code was extracted
from its Markdown block and run in a temporary, differently named Go module:
`ready=true status=200`, with standard-library HTTP mocks and installed process
bundles. Relative documentation targets and `git diff --check` passed. Only
Markdown documentation is changed; no v1 wire or product behavior changed.
No benchmark was rerun, and dedicated native Linux remains BLOCKED.
