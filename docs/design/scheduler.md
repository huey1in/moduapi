# Generic priority / weight / health scheduler

`scheduler.priority-weight-health` is an optional official process. Kernel imports
none of its implementation. No account, provider, quota, credential or billing
entity appears in its wire inputs or policy.

## Algorithm

1. Validate the candidate set/hints; exclude `excluded_ids` on every attempt.
2. Partition by integer priority, larger first. A tier with any positive configured
   weight gives zero-weight candidates no new traffic, even when positive candidates
   are temporarily unavailable. If all non-excluded weights in the tier are zero,
   treat each base weight as one. No historical weight offset/smoothing.
3. Exclude hard cooldown and unavailable capacity. Choose the highest tier with
   remaining candidates. Retry number never indexes a priority array.
4. Look up bounded HMAC-SHA256(route + NUL + affinity) sticky state. A hit must be
   in the chosen tier, not excluded/zero-disabled, outside cooldown, have no
   consecutive failures, and have capacity. Prefer escapes; strict rejects an
   existing unavailable binding. An expired/missing binding starts normally.
5. Otherwise sample within that tier using `base_weight * health_factor`, where
   `health_factor = max(min_weight_factor, 1 - error_EWMA)`. Health disabled gives 1.
   Defaults: alpha .2, minimum .1, three consecutive failures → five-second cooldown.
6. Atomically create a random opaque 16-byte lease and increment reservations under
   the same mutex. Cancellation before return removes that reservation.
7. Report releases first, once; then updates health/timing and binds sticky only on
   success. Cancelled Report releases without learning. Unknown/duplicate/expired
   leases do not update state. Lost RPC responses/reports recover by expiry.

Priority/weight/capacity hints are `moduapi.scheduler.priority` (int64, default 0),
`.weight` (0..1e9, default 1), `.capacity` (0..1e9, default 0 = unlimited). Unknown
metadata is ignored. Malformed known hints reject selection rather than silently
changing policy. Metadata values in YAML must be strings.

## Health and admission

Health is keyed by candidate ID across routes in this process. Successful 2xx/3xx
reports sample zero error; transport/timeout/protocol errors and HTTP401/403/429/5xx
sample one. Other HTTP statuses, downstream/local/plugin errors and unknown failure
kinds are neutral. EWMA first-byte latency, outcome counts, consecutive failures,
last failure/seen, duration and emitted bytes remain plugin-local. Success resets
consecutive failure/cooldown and gradually reduces the error EWMA. Cooldown expiry
permits another weighted attempt, without a complicated half-open state machine.

Capacity checks snapshot in_flight **plus** plugin reservations. This deliberately
double-counts overlapping active leases: it prevents oversubscription from concurrent
snapshots at the cost of utilization. It is not a distributed lock or protection
against independent traffic. Configure `lease_ttl` longer than the gateway request
timeout, including maximum streams. No waiting queue is implemented.

Every outcome can release a lease, including neutral cancellation/errors. A lease
stores candidate/route, created/expires and only a hashed affinity key. Its bytes
contain no business information. Duplicate feedback cannot decrement twice or
poison health twice. Cleanup expires leases, sticky entries and idle health state;
the interval is bounded at 1ms..30s (default 1s). All tables have explicit limits.
Expired admission recovery may lag by one cleanup tick. State pressure also reclaims
expired entries synchronously. Restart clears local history/bindings/reservations;
the host's current in_flight snapshots still apply.

## Sticky and reasons

RequestInfo metadata `moduapi.affinity.key` is opaque and limited to 512 bytes;
oversized values are ignored. Raw keys are never stored or logged. Per-process
random HMAC prevents low-entropy offline lookup of retained hashes. Route ID scopes
bindings. The OpenAI protocol example explicitly maps ingress `X-Moduapi-Affinity`
to this hint and does not forward that header upstream. It is a caller hint, not
identity/authentication. Scheduler never reads protocol JSON.

Modes: off; prefer (default); strict (only for deliberately immovable sessions).
TTL defaults to one hour, refreshed by successful feedback. TTL + max_entries
(10000 default) + LRU bound the cache. Prefer escapes unavailable capacity/health;
the corresponding flags can opt into rejection. Exclusions and priority always
win. Failure never creates a binding; successful fallback may move one.

Reasons include priority_weight, health_penalty, capacity_fallback,
priority_fallback, sticky_hit and sticky_escape_{unhealthy,capacity,excluded,
priority,weight,missing}. Reasons contain no raw key, URL, secret or request data.
`strict`/disabled escape returns local `sticky_unavailable` instead of silently
binding elsewhere. Reasons explain the decisive branch, not every scoring detail.

## Example and boundaries

Run two mocks as in the README, then `moduapi serve --config examples/advanced.yaml`.
Plugin options live under `plugins.config.scheduler.priority-weight-health`.
See [ADR 0007](../adr/0007-scheduler-policy-and-admission.md) for admission limits.

| Concern | Mature integrated gateways | ModuAPI |
|---|---|---|
| Inventory | New API channel / Sub2API account and persistence | Router candidate set |
| Selection | priority, weight, health, affinity alongside business state | Scheduler process, opaque hints |
| Replay | Often coupled to selection/priority progression | Kernel policy + exclusions |
| Health/sticky storage | Host/service/cache/database concerns | Bounded plugin-owned memory |
| Billing/credential/quota | Interacts with eligibility in integrated products | Absent; future independent plugins |

ModuAPI redistributes mature capabilities across explicit boundaries. It does not
fork those projects or merely remove their pages. Historical smoothing, business
auto-disable, sticky waiting queues, token refresh, quotas and persistence are not
copied. Benchmarks are local policy costs, not gateway throughput claims.
