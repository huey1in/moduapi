# ADR 0010: Validate composition and execution placement before expansion

Status: accepted for Phase 3A. Date: 2026-10-17. Baseline: `71d69e6`.

## Problem

The process contract and scheduler can be correct while still costing more than
an integrator is willing to pay. A second protocol and a Go embedding example
are required to test whether the boundaries reduce real integration work. The
placement must also distinguish a split process topology from a bundle and an
in-process control; a direct residual cannot be called an isolated RPC cost.

## Decision

Keep the three typed v1 services and the existing protobuf unchanged. Add only:

1. a Python Anthropic Messages reference process and real process conformance;
2. an experimental `embedding` package whose host owns the listener;
3. an audit-only placement package and Linux harness for split/bundle/local
   comparison;
4. raw result records, integration inventory and an explicit exit gate.

The current result is **CONDITIONAL GO**. Continue only to obtain independent
integration requirements and a dedicated-host repeat. No feature expansion or
new PluginKind follows from this decision.

## Alternatives

* **Continue product work:** rejected because the audit has not shown demand or
  acceptable placement cost.
* **Collapse all services into the host:** rejected as an experiment outcome;
  it would erase the cross-language replacement claim before measuring it. The
  local control remains a comparison, not a production mandate.
* **Require one process per service:** rejected; the existing contract already
  supports a multi-service bundle and the measurements show why it matters.
* **Stop immediately:** retained as the explicit gate outcome if adopters do
  not require the boundaries or the dedicated-host result fails.

## Trade-offs

The second protocol demonstrates no OpenAI-specific Core leakage, but its
implementation still pays the cost of a wire contract and lifecycle server.
Embedding removes the reference executable/listener requirement but does not
remove process startup, configuration, credentials, or host operations. Bundle
placement reduces process/RSS/FD overhead without removing serialization. The
public API remains experimental until an external host uses it.
