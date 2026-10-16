# ADR 0002: Independent processes and a versioned wire API

Status: accepted. Date: 2026-10-16.

## Problem

Go `.so` plugins bind deployment to compiler/build details and do not isolate crashes. A multi-language ecosystem needs a public transport and explicit lifecycle.

## Decision

Use `moduapi.plugin.v1` protobuf/gRPC services over authenticated loopback TCP. The host executes package-relative binaries directly, negotiates identity/version/provides/capabilities, configures them, checks health and owns shutdown/reaping. Stdout is one bounded bootstrap JSON record; stdin is a parent-liveness pipe. Environment inheritance is restricted. Manifests declare network, storage and secret permissions.

## Alternatives

Go plugin `.so` cannot meet language/platform isolation needs. In-process Go interfaces are useful SDK contracts but insufficient as the primary runtime. HashiCorp go-plugin supplies mature machinery but adds another framework/dependency and bootstrap protocol. WASM could offer stronger isolation but restricts language/runtime compatibility and streaming I/O in this first slice. Unix sockets are useful later, but loopback is portable to the Windows development environment.

## Reason

gRPC supplies deadlines, cancellation, multiplexing and bounded stream flow control across languages. A small explicit bootstrap avoids coupling the API to a Go-only framework. API/package version changes protect wire compatibility; plugin and host semantic versions are checked independently.

## Trade-offs

There are IPC copies, child startup cost and more transport lifecycle code to test. The initial implementation has no OS sandbox, mTLS, binary signatures, automatic restart backoff or installer. A per-process bearer token is not a boundary against a hostile local administrator. SDK stdin liveness is cooperative and does not contain arbitrary descendants. See the security contract in the developer guide.
