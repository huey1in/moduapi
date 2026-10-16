# ADR 0001: A runtime-only kernel

Status: accepted. Date: 2026-10-16.

## Problem

A gateway can accumulate accounts, credential management, databases, billing and UI until every installation must run an entire business platform. Removing pages does not remove the runtime coupling.

## Decision

Core owns HTTP serving, lifecycle, routing, generic framing/errors, cancellation, hook invocation, registry SPI and kernel wiring. It contains no database, Redis, users, API-key management, auth implementation, quota or billing. Storage is an experimental SDK key-value contract, not a business repository. Auth and usage have extension points without built-in implementations.

## Alternatives

An embedded SQLite schema would simplify future local features. A minimal built-in auth/billing subsystem would make the initial product feel complete. Both make those business concepts permanent host dependencies.

## Reason

The kernel must run with no storage or business plugins. Auth implementation belongs to an authenticator plugin; prices and balances belong to billing plugins. The kernel enforces lifecycle safety independently of those policies.

## Trade-offs

The first slice has no ingress auth or durable usage. It binds to loopback by default. Integrations need an existing authenticated edge until the authenticator wire service exists. This keeps the kernel boundary explicit instead of hiding a business platform in generic interfaces.
