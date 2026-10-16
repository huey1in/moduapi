# ADR 0005: Candidate routing, selection and replay are different policies

Status: accepted. Date: 2026-10-16.

## Problem

A combined routing/retry selector is difficult to extend with priority, weight, health, capacity, queues or sticky escape. Blind retries can duplicate billable work and sticky pins can repeatedly choose failed candidates.

## Decision

Router returns a candidate set. Scheduler plugins select and report results using public candidate facts, request metadata, attempt number, exclusions and optional leases. Replay is controlled by an independent conservative policy, disabled by default and bounded by attempts/candidates. The host rejects out-of-route and excluded selections. No priority/weight/health formula is built into Core.

## Alternatives

Random selection in Core is easy but not an appropriate universal strategy. A kernel priority-weight formula would satisfy one policy while limiting others. Letting the scheduler execute retries mixes replay safety with scoring and makes stream commitment harder to enforce.

## Reason

Mature gateways need highest-available priority tiers followed by within-tier weighting, health feedback and sticky escape. Those require extensible metadata and feedback, not a business channel model. A future policy plugin must still obey the kernel's commitment/cancellation limits.

## Trade-offs

Only first-eligible and process-local round robin ship now. Weighting, shared health, queue reservations, sticky key extraction and distributed leases remain future scheduler work. Feedback is bounded best effort; concurrency plugins must expire leases to recover from lost reports. Replay opt-in may duplicate upstream work, and this slice makes no same-candidate replay or exactly-once promise.

## Phase 2 implementation update

The boundary remains accepted. The optional advanced scheduler now implements
weighting, health feedback, bounded local reservations and sticky state, as specified
in [ADR 0007](0007-scheduler-policy-and-admission.md). Distributed leases and waiting
queues remain deferred. The original trade-offs above describe the Phase 1 delivery.
