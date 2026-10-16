# Scheduling lessons turned into constraints

Reviewed against the same current upstream heads on 2026-10-16: New API
`d032204733db31208f10bac118b6e683aab3bf29`, Sub2API
`b0a02a3304a0a78cdd003ddda1dd23ba02c03084` (GitHub API rechecked).
This records engineering failure modes and our constraints, not claims that every
listed upstream currently has each bug. [Original research](../research.md) gives
the source inventory and license review. No code or contracts were copied.

1. **Consistent weights.** New API's
   [channel cache](https://github.com/QuantumNous/new-api/blob/d032204733db31208f10bac118b6e683aab3bf29/model/channel_cache.go)
   includes historical all-zero/tiny-weight handling. ModuAPI specifies zero cases
   directly and statistically tests positive ratios without hidden offsets.
2. **Retry index is not priority index.** Dynamic eligibility/cooldown can change
   the tier set. Each Select gets all candidates and exclusions, never an index
   into yesterday's priority array.
3. **Sticky cannot bypass capacity.** Sub2API's
   [account scheduler](https://github.com/Wei-Shaw/sub2api/blob/b0a02a3304a0a78cdd003ddda1dd23ba02c03084/backend/internal/service/openai_account_scheduler.go)
   illustrates how sticky, load and admission interact. Prefer immediately usable
   affinity; otherwise use the pool. Do not concentrate ordinary HTTP into a queue.
4. **Sticky cannot bypass health.** Cooldown/exclusions stay authoritative, even
   on a cache hit. Strict mode reports unavailability explicitly.
5. **Downstream cancellation is neutral.** A user leaving a stream says nothing
   about candidate health. Keep cancellation distinct through Python/gRPC/HTTP.
6. **Snapshots are not admission.** Reading 9/10 twice does not reserve two slots.
   Capacity check and lease creation must be one plugin-local atomic operation.
7. **Release is idempotent.** Success/error/cancel, duplicate delivery and lost
   feedback are normal. Release once by opaque lease, recover loss through TTL.
8. **Unknown/stale state needs explicit behavior.** Invalid hints reject; unknown
   errors are neutral; new candidates begin unpenalized; stale idle health expires;
   active leases prevent health-state eviction. No permanent disable from a stale
   sample, and no optimistic capacity inference from missing acknowledgements.

These are generic constraints. We deliberately omit account tiers, OAuth refresh,
quota windows, cost, distributed locks and database persistence. Their semantics
cannot be inferred from a generic Candidate.
