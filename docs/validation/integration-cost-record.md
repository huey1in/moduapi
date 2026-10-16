# Participant-owned integration cost record

Copy one record per participant and scenario. Empty fields mean **UNKNOWN**,
not zero. Public leads start `UNVERIFIED`; only the participant can confirm
their current requirement and review the recorded result. Never include keys,
request bodies, private endpoints or raw affinity values in a public record.

## Requirement and independence

| Field | Participant response / evidence link |
|---|---|
| Public project, participant/role, date; consent to publish result | |
| Cohort: component author / embedding host; independence from ModuAPI authors | |
| Current workload and production/nonproduction use | |
| Required protocol / scheduler / transport substitutions; why each is needed | |
| Required implementation languages and separate release/crash boundaries | |
| Why existing configuration/hooks are insufficient; concrete failing example | |
| Best existing alternative, version/commit and existing team expertise | |
| Need an HTTP Kernel or only an LLM client/router? | |
| Body/response sizes, QPS/concurrency, TTFB and p95 budget, CPU/RSS limits | |
| Acceptance tests agreed **before** implementation | |

## Same-scope effort, including the work ModuAPI does not remove

Record commits/files and timed work rather than asking for a promotional
estimate. Alternate implementation order or disclose learning bias. Use the
same protocol subset, traffic, errors, cancellation and shutdown cases.
Existing reused functionality counts as reuse, not as code that must be rewritten.

| Work item | Best alternative: actual / estimate / unknown | ModuAPI: actual / estimate / unknown | Evidence and maintenance owner |
|---|---|---|---|
| Handwritten semantic component code | | | |
| Handwritten adapter/glue code (exclude component semantics) | | | |
| Generated bindings and dependency/tool setup (separate LOC) | | | |
| Public types/interfaces/contracts the author must understand | | | |
| Host internal modules imported or edited; fork/patch needed? | | | |
| Lifecycle/supervision/cancellation/error mapping code | | | |
| Test code and independent behavior cases (not test file count) | | | |
| Configuration/deployment manifests/services/processes | | | |
| Time to first passing request; debug time; total engineer-hours | | | |
| Component replacement steps and files edited | | | |
| Upgrade trial from pinned old version to chosen new version | | | |
| Profiling/operations: logs, traces, crash recovery and on-call steps | | | |
| Long-term compatibility promises and expected test matrix | | | |
| Listener/TLS/edge auth/secrets/admission/configuration work left to host | | | |

For LOC, record counting tool/version and treatment of comments/blanks/tests.
Net saved maintained glue = alternative glue minus **(ModuAPI glue plus any
new maintained bridge/tooling required by ModuAPI)**. Report negative savings.
Also report the total integration effort; a shorter adapter is no saving if
debugging, packaging or upgrades cost more. A task checklist is not measured LOC.

## Outcome and review

| Field | Participant-reviewed answer |
|---|---|
| Same-scope functional tests passed/failed; logs/commit links | |
| Performance and process-resource results, native host provenance | |
| Required boundary missing in the alternative, if any | |
| Actual saved hours / maintained LOC / avoided upgrade work | |
| Which costs outweigh the benefit; would choose the alternative? | |
| Is there a real intended use, owner and next integration milestone? | |
| Participant confirmation URL/date or private confirmation location | |
| Status: UNVERIFIED / CONFIRMED REQUIREMENT / MEASURED TRIAL / REJECTED | |

Confirmation of a requirement is not confirmation of savings or adoption.
Project names, public comments, stars and our own runnable examples cannot fill
the last two rows. The [exit gate](phase-3a-exit-gate.md) requires two independent
written requirements **and** measured benefit; this template changes no threshold.
