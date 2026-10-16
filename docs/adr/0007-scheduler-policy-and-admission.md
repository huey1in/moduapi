# ADR 0007: Plugin-owned scheduling, conservative admission and expiring leases

Status: accepted. Date: 2026-10-16.

## Problem

Priority, weight, health and sticky preferences can silently couple to retries and
business entities. Load snapshots do not atomically admit concurrent requests.
Cancelled or lost Select responses and duplicate Report calls can leak/corrupt slots.

## Decision

Implement one generic process-local scheduler plugin. Keep Candidate metadata as
`map<string,string>` and standardize only three scheduler hints and an opaque
affinity hint. Router supplies the whole set; kernel owns retry safety/exclusions;
plugin owns priorities, sampling, health, TTL/LRU, leases and atomic reservations.
Select and reservation share one mutex. Opaque random 128-bit leases are returned
for every selection; release is idempotent and expires. Only successful feedback
binds HMAC-hashed affinity. Sticky respects priority, exclusions, health and capacity.

Conservative capacity admission checks **observed in_flight + live reservations**.
These overlap after the host starts a leased attempt; v1 lacks an acknowledgement
that would distinguish pending from active reservations. We accept underutilization
rather than infer overlap and accidentally admit more work from stale snapshots.
All traffic must use this scheduler process for its reservations to be authoritative.
Lease TTL must exceed the maximum request lifetime; default 6m covers kernel default
5m. Expiry is recovery, not revocation of a still-running upstream request.

Transport/timeout/protocol errors and HTTP 401/403/429/5xx are failed health samples.
400/404, local rejection, plugin error and downstream cancellation are neutral.
A cancelled client is not evidence about an upstream. No credential invalidation,
refresh, quota or permanent auto-disable is inferred. Unknown errors are neutral.
Unknown candidates begin unpenalized; stale idle health expires, not permanently
disabled. Invalid hints and state limits fail closed with explicit local errors.

## Alternatives

A kernel health map breaches ownership. `max(snapshot,reservations)` can oversubscribe
external in-flight work. Queueing behind sticky concentrates traffic and requires
another fairness policy. Distributed admission requires a separate state provider.
Admitting via a new mandatory RPC would break v1; a future additive acknowledgement
can improve utilization without changing current conservative safety.

## Reason

The kernel needs composable selection and feedback, not a universal score or account
model. Short cooldown plus gradual EWMA recovery is sufficient for this stage.

## Trade-offs

Local state resets on restart; leases are not durable. Lost reports recover by TTL,
not exactly-once RPC. Select checks cancellation before/after reservation, but a
response lost after the method returns still requires expiry. Capacity is conservative
and no distributed guarantee is claimed. One mutex and linear candidate scans are
intentionally simple; benchmarks quantify cost before introducing indexes/shards.
