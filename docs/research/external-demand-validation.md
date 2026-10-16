# External demand validation: Phase 3A closeout

Reviewed 2026-10-17 against ModuAPI `dfde6e6`. Decision: **CONDITIONAL GO for
evidence collection only**. Confirmed external component authors: **0**.
Confirmed external embedding hosts: **0**. No Issue, PR, email, discussion reply
or private message was sent during this review.

## What this investigation can establish

Public issue bodies, comments and project-owned technical plans were inspected,
including author, creation date, resolution and suggested alternatives. GitHub
issue metadata was checked through its API, rather than inferred from search
engine relative dates. Dates below are UTC. Search covered APISIX/its Go runner,
Envoy, Bifrost, LiteLLM, llm-d-router and go-plugin, plus Go library proposals.
Search terms included custom protocol/body conversion, custom routing/scheduler,
multiple language runners, library embedding and host-context dependencies.
This is a bounded public-source review, not an exhaustive market survey.

`UNVERIFIED` means **not confirmed as a ModuAPI requirement by the person
concerned**. It does not mean that the public issue is imaginary. An issue
confirms what its author reported at that time; it does not confirm current
need, willingness to migrate, budget, or a need for all three typed services.
Project-maintainer plans and secondhand reports are identified separately.
Nothing here is an adopter endorsement.

## Traceable leads and counterevidence

| ID / original source | Published / proposer | Concrete problem and existing alternative | Relevance and verification status |
|---|---|---|---|
| D1 [APISIX #10064](https://github.com/apache/apisix/issues/10064) | 2023-08-21; `cod-soap` | A multilingual team wanted Go, Java and Python plugin runners together. Array-valued runner configuration failed. A maintainer explained that APISIX managed one runner executable; author acknowledged the answer on 2023-09-07. Alternatives: one supported runner/language, Lua plugins, or independently operated services with an adapter. | Strong lead for independent language boundaries; old and closed, not evidence the team still needs them. It says nothing about replacing an AI protocol or scheduler. **UNVERIFIED**. |
| D2 [LiteLLM #25297](https://github.com/BerriAI/litellm/issues/25297) | 2026-04-07; `enniomaldonado-ai` | Wants programmable composite selection, fallback/backoff and experiments in the proxy, rather than duplicating routing outside it or abandoning the policy. SDK custom routing is the author's stated alternative. [Mavyre's 2026-06-10 comment](https://github.com/BerriAI/litellm/issues/25297#issuecomment-4673823610) describes cheapest-then-next-on-error selection. Issue was open when inspected. | Strong lead for a complete scheduler extension, not proof that a separate process or another language is required. Cost policy is not a feature request for ModuAPI. **UNVERIFIED**. |
| D3 [ShenYu #7351](https://github.com/apache/shenyu/issues/7351) | 2026-09-28; `hengyuss` | Maintainer task says OpenAI/Spring AI DTOs, SSE encoding and retry/invocation are coupled; new protocols change the main flow and unknown fields can be lost. Proposes separate protocol/provider/transport contracts, citing APISIX, with linked implementation tasks. | Direct architectural extension problem, but a project-owned implementation plan, not an independent migration request. Its planned in-process Java solution may be cheaper than ModuAPI. **UNVERIFIED**. |
| D4 [go-plugin #334](https://github.com/hashicorp/go-plugin/issues/334) | 2025-03-03; `pidanou` | A Python plugin author cannot work out callbacks into the Go host/broker. `kamrankamilli` asks for the same on 2025-06-25. Alternatives: implement the broker protocol, use an explicit application RPC service, or keep callbacks in Go. Issue was open. | Actual cross-language integration friction. ModuAPI has no Host Capability API, so it currently **does not solve this requirement**. No AI gateway requirement stated. **UNVERIFIED**. |
| D5 [Envoy #23919](https://github.com/envoyproxy/envoy/issues/23919) | 2022-11-09; `rjhuijsman` | Wants arbitrary filter logic to rewrite a Kubernetes upstream destination. Header rewrites initially left the cached route unchanged. Maintainers recommended `clear_route_cache`; the author [confirmed resolution](https://github.com/envoyproxy/envoy/issues/23919#issuecomment-1310741821) on 2022-11-10. | **Counterevidence** to an unmet routing gap: an existing extension solved it. No new Kernel needed for this stated scenario. **UNVERIFIED** as a ModuAPI lead. |
| D6 [Bifrost #249](https://github.com/maximhq/bifrost/issues/249) | 2025-08-09; `Pratham-Mishra04` | Maintainer describes provider enum/instance/storage constraints preventing multiple compatible endpoints. Listed workarounds included a fork, another proxy, or separate deployments. Closed completed 2025-08-29. | Historical coupling evidence, **not a current missing capability**. Current custom-provider configuration is an alternative; no need to replace the gateway proven. **UNVERIFIED**. |
| D7 [Bifrost #2202](https://github.com/maximhq/bifrost/issues/2202) | 2026-03-22; `akshaydeo`, relaying `joseg_28742` from Discord | A custom cost-metrics plugin wants the host's model catalog/provider object; creating another instance was the mentioned workaround. Maintainer proposed adding it to context. The pinned core source now stamps the catalog onto context, although the issue remained open. | Secondhand public transcript, not a direct interview. Demonstrates host-service coupling, **not** demand for a business-free host. ModuAPI intentionally has no such catalog service. **UNVERIFIED**. |
| D8 [llm-d-router #896](https://github.com/llm-d/llm-d-router/issues/896) | 2026-04-27; `questcollector` | Wants injectable Go HTTP handlers for prefill/decode/encode sidecar paths. Comments identify hidden prefill token accounting as the motivation. [Closing comment, 2026-10-01](https://github.com/llm-d/llm-d-router/issues/896#issuecomment-5923079423) says accounting was fixed in #916 and extensibility moves to a coordinator. | A real handler-injection request, resolved/reframed within llm-d. Specialized disaggregated inference is outside ModuAPI's current use case. **UNVERIFIED**. |
| D9 [go-plugin #326](https://github.com/hashicorp/go-plugin/issues/326) | 2024-12-28; `keshavbhattad` | Reports increased pod memory after migrating an in-house application from native Go plugins to go-plugin, recognizing per-process cache/IPC costs. Requests an existing remedy; no measured numbers supplied. | **Cost counterevidence**, not an adopter prospect or competitor benchmark. Alternatives include in-process placement, sharing processes, or retaining the existing implementation. **UNVERIFIED**. |
| D10 [vectorless-engine Go-library plan](https://github.com/hallelx2/vectorless-engine/blob/13db104ea165af01cd0bc563ba359461d756e492/docs/LLMGATE.md), [introducing commit](https://github.com/hallelx2/vectorless-engine/commit/13db104ea165af01cd0bc563ba359461d756e492) | 2026-04-19; `hallelx2` (commit author identity; document has no separate byline) | A retrieval engine wants an independently consumable Go LLM client/router and describes extracting it, using langchaingo adapters. It criticizes Bifrost's framework/type worldview. This is a library design and author opinion, not our finding that Bifrost cannot embed. | Adjacent embedding lead with its **own alternative**. It does not request an HTTP Kernel or external processes. This Go `hallelx2/llmgate` is **not** the Rust `devstroop/llmgate` audited earlier. **UNVERIFIED**. |

For D7's implementation countercheck see Bifrost
[`setModelCatalogOnContext`](https://github.com/maximhq/bifrost/blob/767bd89a639efbe1461d7a653f451124ecd23931/core/bifrost.go).
Do not equate an open issue with an unimplemented feature.

## What remains unsupported

The sources establish separate needs for protocol decoupling, programmable
routing, multilingual extensions and Go library consumption. **No inspected
source establishes the conjunction**: an independent integrator needing all
three replaceable cross-language components, rejecting a reasonable existing
solution, and accepting the measured cost. No source quantifies saved work from
using ModuAPI. No reviewed source proves that removing account/billing types
alone is a migration reason. D10's design preference is weaker than that proof.

The closest potential requirements are therefore hypotheses:

| Hypothesis | Evidence | Plausible cheaper solution / unanswered question |
|---|---|---|
| A multilingual team independently releases protocol and routing components | D1 + D3, from **different** people/projects | APISIX runners or Envoy processor; does any one team require both, plus replaceable transport? |
| An operator needs complete scheduler control | D2 | LiteLLM SDK/custom strategy, Bifrost routing hook or EPP plugin; is process independence actually required? |
| A Go product wants reusable routing without a gateway binary | D8/D10 | Bifrost core, langchaingo plus a small router, or the llm-d coordinator; is an HTTP gateway needed at all? |

Combining different people's issues into a synthetic persona would not confirm
any of these hypotheses. The [alternative comparison](../validation/phase-3a-alternatives.md)
identifies what would have to be measured with a real participant.

## Candidate contacts, not confirmed integrators

No contact is authorized by this research task. These are public project
identities only; do not collect private contact details or automatically post.

| Priority / cohort | Candidate | Why a technical conversation could falsify the claim |
|---|---|---|
| 1 / component author | `cod-soap`, APISIX D1 | Confirm whether multilingual runner need survives and whether independent lifecycle is worth a second runtime. |
| 2 / component author | `enniomaldonado-ai` (or `Mavyre`), LiteLLM D2 | Determine whether a normal Python routing extension already suffices. These commenters are **not two independent validation cohorts**. |
| 3 / component author | `hengyuss`, ShenYu D3 | Ask whether protocol separation requires external execution at all; their current Java solution may settle it. |
| 4 / embedding host | `hallelx2`, vectorless-engine D10 | Ask whether they need an HTTP Handler/runtime rather than a provider client library. A “no” disqualifies the lead. |
| 5 / embedding host | `questcollector`, llm-d D8 | Check whether the coordinator resolved the need and whether generic embedding would lose required inference semantics. |

Confirmed participants remain **none**. `pidanou` is an informative source on
broker usability, not an AI gateway prospect without further evidence.

## Bounded next decision

The public review is complete; interviews, participant-owned integrations and
a dedicated Linux machine are unavailable in this session. The original gate
is not passed by these documents. Proposed evidence window: **2026-10-17 to
2026-11-07**, at most five candidate projects, with contact only after explicit
authorization and at most one agreed follow-up. This is a review deadline,
not authorization to message anyone or continue automatically.

At that review, require the two original independent written requirements and
measured integration benefits, plus the original Linux gate. If neither cohort
has a qualifying requirement after the bounded validation, choose **STOP**.
If only one qualifies, GO still fails; consider only an evidence-backed narrow
pivot or cessation, not an automatic feature backlog or rolling extension.
Silence is neither consent nor proof that nobody in the market needs the idea.
Environment-only delay must be recorded separately from absent demand.
