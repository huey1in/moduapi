# ADR 0008: Bound supervision and retain only safe stderr events

Status: accepted. Date: 2026-10-16.

## Problem

A dead process must disappear from readiness immediately, but unbounded restart
loops waste resources. Raw stderr can contain credentials, bodies or affinity keys
that cannot be reliably removed by regexes.

## Decision

Supervise only plugins that reached ready. Unexpected exit closes old resources,
waits bounded exponential backoff (250ms initial, 30s cap, ±20% jitter), then fully
starts/negotiates/configures/health-checks a replacement and atomically publishes it.
Five crashes in a rolling 60s stop automatic restart. Replacement startup failure
also stops supervision. Intentional stop/restart/shutdown does not count. Explicit
restart resets protection. Proxies resolve the current process per call; bindings
cannot change. Runtime options are configurable by embedders, not new kernel policy.
Close cancels backoff/startup, serializes mutations, stops children and joins watchers.

Retain a 64KiB stderr tail per process, with a 4KiB input line cap. Accept only JSON
records with `event` and optional `code` from closed vocabularies. Drop free text,
extra fields, invalid JSON, oversized lines and unknown values. SDK emits safe
lifecycle records. Diagnostics are available through the process/manager API; no
daemon-control API or dashboard is introduced.

## Alternatives

Never restarting loses availability. Retrying failed startup indefinitely hides
deployment errors. Publishing an unready process exposes partially configured
clients. Raw stderr with token redaction cannot protect an unlabeled body/session
key; retaining it privately still creates a disclosure surface.

## Reason

Bounded recovery preserves the existing registry model. Closed-vocabulary events
make the no-secret diagnostics guarantee enforceable even for accidental plugin logs.

## Trade-offs

No arbitrary Python traceback/free-text tail is retained. This is a deliberately
restricted stderr tail, not a general log collector. Crash-loop state is local and
resets with the host. All installed plugins are currently required for readiness.
No hot reload, package replacement, process-tree containment or OS sandbox exists.
