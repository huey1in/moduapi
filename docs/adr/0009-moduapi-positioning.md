# ADR 0009: Validate a gateway construction kernel before expanding it

Status: accepted for a bounded validation phase; broader product Go remains
conditional. Date: 2026-10-16. Audited baseline: `a3f011d`.

## Problem

Two implemented phases establish that ModuAPI can forward requests and manage
components. They do not establish that another AI gateway or plugin platform is
needed. Independent processes, protobuf/gRPC, manifests, health, sticky routing
and weighted selection are established mechanisms. A provider-free host can still
be an expensive and unnecessary reimplementation of existing projects.

The [source-based audit](../research/competitive-architecture-audit.md) compares
eight alternatives across thirty dimensions. The [E2E measurements](../benchmarks/end-to-end.md)
measure the real process topology; local scheduler benchmarks are insufficient.
Existing ADRs are design history, not proof that their trade-offs remain justified.

## Decision

Choose **Option A, narrowly and conditionally**: ModuAPI is an experimental,
composable **AI HTTP gateway construction kernel for trusted operator-installed
components**. The executable is one reference assembly of the kernel. It is not
yet a stabilized embedding library or a proven general-purpose gateway product.

Continue only with positioning, interoperability, embedding and execution-placement
validation. Suspend Auth/Secret/Middleware/Usage and all new plugin-kind work.
There is no new production feature, protobuf revision or mass refactor in this ADR.
The current v1 ownership/cancellation/error semantics remain supported.

### What it is

A host that composes typed Protocol, Scheduler and Upstream services while owning
HTTP lifetime, routing to candidate sets, replay/commitment safety, cancellation,
deadlines and stream cleanup. Provider payload interpretation and policy state
remain replaceable outside that host. The host knows HTTP, models, generic
candidates, stream framing and token-shaped usage summaries. Calling it completely
domain-agnostic or a universal protocol runtime would be inaccurate.

### What it is not

It is not an account gateway, business platform, authentication product, billing
system, admin console, distributed scheduler, provider catalog, full protocol
translator, untrusted-code platform or plugin marketplace. It is not a replacement
for Envoy's HTTP stack or llm-d's inference scheduling expertise. “Lightweight”
currently means a small mandatory business-domain surface, not a demonstrated
latency/RSS/deployment advantage.

### Why it might deserve to exist

The testable integration proposition is independently replacing **entire protocol,
transport and selection implementations** through one versioned contract, without
requiring provider/account/identity/billing objects in the host. Component authors
can use a process language independently of the host and can keep policy state
with the component. The Go/Python fixture demonstrates wire/stream ownership;
architecture tests demonstrate import separation. Neither proves market demand.

**ModuAPI should exist because an integrator who must independently replace and
release protocol, transport and selection components across languages can do so
without adopting the host's provider/account/billing model, provided the measured
isolation cost is acceptable and real integrations demonstrate less work than
extending an existing gateway.**

This sentence is a falsifiable condition, not a claim of global novelty. No single
mechanism is a competitive innovation. The specific combination differs from the
inspected boundaries; that combination still needs independent users.

## Differences from the reviewed alternatives

| Alternative | Real difference and when to choose it instead |
|---|---|
| New API | Task plugins already extend native routes, host protocols, stream events and usage; atomic generations and marketplace management are beyond ModuAPI today. Host channel/task/pricing/identity ownership remains. Choose New API for that complete operating model; ModuAPI only if replacing core components without that model is mandatory. |
| Sub2API | Independent gRPC transport, manifests, config validation, package integrity and host callbacks already exist. Its inspected capability scope and request/HostService expose accounts, OAuth/platform and outbound identity. ModuAPI's generic candidates and separate selection replacement avoid that mandatory domain. Choose Sub2API for account/OAuth operations. |
| APISIX | External multi-language runners, ordered phases and host callbacks already exist; AI proxy plugins already select upstreams. Its mature generic proxy remains preferable when one is already deployed. ModuAPI must prove that a small embedded assembly contract saves integration work; being written in Go is not sufficient. |
| Envoy/ext_proc | A mature generic data plane can externalize HTTP processing, including streamed/full-duplex bodies. ModuAPI additionally owns component spawning/negotiation and exposes typed whole-scheduler/upstream services. Those responsibilities are also overhead. Prefer an ext_proc service when Envoy can remain the data plane. |
| Bifrost | Already a usable core library plus gateway/framework, with explicit LLM/HTTP/MCP hook ordering. Its host owns providers, typed AI operations and keys. ModuAPI externalizes those component implementations instead of providing a broad in-process provider library. Choose Bifrost for ready provider functionality or efficient Go embedding unless independent components are a hard requirement. |
| llm-d EPP | Filter/Score/Pick/Profile and request/data extension points are more granular than Select/Report. EPP is an external processor with Go scheduling plugins and an inference/Kubernetes endpoint model. ModuAPI is neither a new scheduling algorithm nor an alternative for cache-aware cluster serving; its scope is generic opaque candidate replacement. |
| llmgate | ProtocolAdapter and NeutralRequest/Response already separate protocols with rich content/tool/stream representations. ModuAPI carries opaque bytes and avoids a universal chat DTO in the host, at the cost of not delivering those semantic translations. Choose llmgate for its translation/privacy use case. |
| LiteLLM | Broad SDK/router/proxy, callback hooks, cooldown, routing and retry/fallback already exist; DB/Redis are not mandatory for local SDK use. Its host owns common AI/provider/cost/proxy concepts. Choose it for broad integration coverage rather than reconstructing that product here. |

Evidence, exact revisions and qualifications are in the audit matrix. These are
different trade-offs, not assertions that the compared projects are poorly designed.

## Boundary implications

Retain the three typed v1 services. Separate ownership does **not** mandate a
separate process per service: multi-service bundles are already supported and used
by Python conformance. The official minimal topology uses four processes and six
business RPCs per ordinary success; stock discovery starts six processes because
it starts all five official bundles. The E2E increment is the whole slice's cost,
not a causal measurement of protobuf or RPC alone.

Do not grow an enum of business features. Reserved kind names/experimental SDK
sketches are not committed APIs. Future extension points, if justified, need typed
versioned input/output plus explicit phase/order/failure/ownership semantics.
Do not introduce a generic `Call(kind, bytes)` service or pipeline language now.

Do not add HostService now. A future optional, independently versioned bounded
diagnostic/correlation capability may be justified by a real integration; it must
avoid callback deadlock and request-lifetime extension. Config already has delivery;
network/pools remain executor-owned. Accounts, credentials, secrets and persistence
are not smuggled into a universal host-services object.

The threat model is trusted extensions installed by the operator, running as the
host OS user. Manifest permissions are declarations/cooperative grants. Fault
isolation is not confinement. Untrusted download-and-execute is out of scope and
would require a new threat model before any marketplace/security promise.

## Alternatives

**Option B — standalone plugin-runtime pivot:** retain process/wire/conformance
work, remove standalone HTTP/router/retry product responsibility, expose a
host-neutral runtime and write a requested adapter for an existing proxy. This
currently has *less* differentiation: go-plugin, APISIX runners and ext_proc cover
most of the mechanisms. Do not choose it merely because runtime code exists. If
one adopter needs an integration runtime rather than a gateway kernel, specify that
host's actual requirements first and then decide what to extract/discard.

If that pivot is later justified, retain lifecycle/bootstrap, bundle resolution,
supervision, bounded diagnostics and the compatibility/ownership test corpus.
Remove the standalone server/router/retry assembly from the runtime product;
official AI components become examples or a separate reference gateway. Split
lifecycle/discovery contracts from AI Request/Usage and the three service bindings:
today's registry only accepts those three roles, so it is not already a generic
component registry. Specify versioned service descriptors against the requested
host before changing that API. An Envoy adapter must respect Envoy-owned transport
and retry, not tunnel Execute around its router. This is a new integration design,
not a rename or a reason to implement a generic bytes dispatcher now.

**Option C — Stop:** archive the prototype if no independent integration need or
acceptable execution placement emerges. Use Bifrost/LiteLLM for SDK/gateway work,
New API/Sub2API for business/account gateways, APISIX/Envoy for proxy extensions,
llm-d for inference scheduling, llmgate for neutral translation. Preserve research,
benchmarks and conformance failure cases; suitable generic regression tests could
be contributed upstream after project/license review. Sunk cost is not a counterargument.

## Reason

The code establishes a specific contract combination worth testing. It does not
establish a reason to compete in feature count, market distribution, raw proxy
performance or generic process supervision. A short validation phase can falsify
the remaining proposition more cheaply than adding auth, usage and administration.

## Trade-offs and exit gates

There is no new feature milestone while positioning is unresolved. Require two
concrete integration specifications (component author + embedder), a second
independent protocol without kernel edits, a public-package embedding experiment,
and reproducible Linux E2E/profile/slow-consumer/cancellation measurements. Compare
actual integration effort against existing alternatives. See the roadmap for
proposed performance/resource gates; those are engineering hypotheses requiring
user/workload confirmation, not promises established by this Windows run.

If process isolation provides no required benefit, or an acceptable profile cannot
be demonstrated, choose Stop or a specifically justified host adapter. No automatic
return to Auth/Secret/Middleware/Usage follows this audit. The core competence to
validate is **safe, replaceable component composition**, not a new weighted scheduler
or a generic plugin ecosystem.

## Phase 3A result

The follow-up validation adds no PluginKind and no protobuf revision. A Python
Anthropic Messages reference process passes the existing Protocol contract, an
external Go package serves a real request through the experimental embedding
surface, and Linux records compare split, bundle and in-process placement. The
checks establish that Protocol/Upstream/Scheduler can be independently replaced
and that a bundle process is a materially cheaper placement than three plugin
processes. They also expose substantial large-body and process/RSS costs; the
residual is the complete gateway path, not an isolated gRPC number.

The decision is **CONDITIONAL GO**, recorded in the [Phase 3A exit gate](../validation/phase-3a-exit-gate.md).
That decision authorizes only two further validation tasks: obtain independent
component-author and embedding-host requirements, and repeat the placement
experiment on a dedicated Linux host with profiles. It does not authorize Auth,
Secret, Middleware, Usage, storage, new plugin kinds, or a stable public API.
If those requirements do not materialize, STOP and retain the research and
regression artifacts rather than expanding the Kernel.
