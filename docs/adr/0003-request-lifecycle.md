# ADR 0003: One cancellation tree and explicit response commitment

Status: accepted. Date: 2026-10-16.

## Problem

Retry, SSE and callbacks can leave upstream work running after disconnect, leak stream senders, or append a second upstream after partial output. HTTP 200 is not proof of protocol completion.

## Decision

Every request has one bounded context tree propagated into protocol, scheduler and upstream RPCs. Attempts own streams and cancellation. Validate start/data/terminal ordering on both RPC response legs. Delay commitment until first body data/end. Never replay after commitment; abort a failed partial response. Apply separate first-byte, idle-read, write and total limits. Scheduler release and post hooks alone receive a bounded independent cleanup context.

## Alternatives

Buffering complete responses makes errors easy to replace but defeats SSE. Detached upstream contexts can recover final billing usage but violate prompt cancellation. Retrying any 200 stream without tracking emission corrupts the response.

## Reason

Cancellation, commitment and completion are independent facts. The kernel must enforce safety regardless of scheduler/provider policy. Upstream status/body errors remain distinct from local errors and transport/protocol failures.

## Trade-offs

Partial responses may end as a transport reset; the host does not fabricate protocol-specific error events. Usage can be incomplete. Once non-stream JSON bytes have been committed, later validation cannot replace them. Exactly-once upstream execution is not promised.
