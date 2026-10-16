# ADR 0004: Protocol representation is separate from HTTP upstream identity

Status: accepted. Date: 2026-10-16.

## Problem

Many providers share the same OpenAI-compatible protocol. Combining protocol parsing, vendor identity and transport produces duplicate adapters and puts vendor fields into the host.

## Decision

Protocol plugins decode ingress, prepare operation/model mapping per attempt, and transform/observe responses. The canonical envelope stays small and carries opaque payloads. The generic HTTP upstream plugin handles base URL, headers, credentials and pooled transport. Per-upstream `protocol_config` is opaque to Core. Future non-HTTP or vendor conversion implementations use separate plugins.

## Alternatives

One provider plugin per vendor duplicates compatible HTTP transport. A universal chat DTO in Core forces every protocol change into the kernel. Pure HTTP passthrough cannot validate protocol completion or isolate future conversion state.

## Reason

Protocol compatibility and provider configuration vary independently. Stream conversion state belongs to one protocol stream RPC rather than request-global kernel DTOs.

## Trade-offs

Response transformation adds an IPC leg and forwarding goroutine. This slice supports only OpenAI Chat-compatible wire data; it does not claim Anthropic/Gemini conversion. Explicit model mapping re-encodes JSON; the common unmapped path preserves bytes.
