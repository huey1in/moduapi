# ADR 0006: Freeze semantics through an independent wire implementation

Status: accepted. Date: 2026-10-16.

## Problem

Go aliases and helpers alone cannot prove a language-neutral contract. Native-only
command resolution excludes interpreters; first-terminal EOF synthesis hides invalid
trailing frames; unstructured gRPC errors lose their meaning.

## Decision

Keep protobuf v1 field numbers/services unchanged. Add a manifest interpreter mode
without relaxing package containment. Resolve interpreter through PATH/absolute
operator configuration; script remains package-local. No shell. Establish the
[normative contract](../plugin-contract.md) and an independent Python reference,
isolated pinned environment and CI job. Validate terminal followed by actual EOF,
map structured Failure details before status fallback, gate SDK admission during
drain and serialize Configure. Exercise unknown wire fields in both directions.

## Alternatives

Go-only tests hide assumptions. Executing arbitrary manifest command text removes
package boundaries. A production Python SDK adds unnecessary scope. New mandatory
RPCs would break old plugins; ValidateConfiguration is deferred.

## Reason

Interoperability depends on lifecycle, ownership and errors as much as field types.
Additive runtime declarations allow Python/Node/Ruby without changing native bundles.

## Trade-offs

Terminal validation waits for RPC EOF under the existing context deadlines. Early
protocol completion cancels rather than drains the source. Trusted interpreters and
processes still share the host OS user; this is not sandboxing. Python conformance
is opt-in locally and required in CI, with a separate dependency surface.
