# ModuAPI

**An experimental kernel for composing AI HTTP gateways.**

**Positioning review:** the current project is an experimental **AI HTTP gateway
construction kernel**. “Lightweight” describes its small mandatory business-domain
surface; it is not a latency, memory or single-binary deployment claim. New plugin
kinds and Auth/Secret/Middleware/Usage are paused for [architecture validation](docs/roadmap.md).
Read the [source audit](docs/research/competitive-architecture-audit.md),
[E2E measurements](docs/benchmarks/end-to-end.md) and [ADR 0009](docs/adr/0009-moduapi-positioning.md).

ModuAPI keeps the HTTP runtime small and runs protocol, upstream and scheduling implementations as independent processes. Install only the capabilities you need. The kernel starts without a database, Redis, users, authentication management, billing or an admin UI.

Phase 2 preserves the first runnable vertical slice and hardens its cross-language contract: OpenAI Chat Completions → router → scheduler → generic HTTP upstream → JSON/SSE. It is an initial implementation, not a production hardening claim. Phase 3A adds an independent Python Anthropic Messages reference, an experimental Go embedding API, and placement validation; it is an evidence-gathering phase, not product expansion.

## Run locally

Requires **Go 1.27.2**. An older Go installation with automatic toolchain downloads enabled can use the `go` directive in `go.mod`. Normal builds use only Go; protobuf generators are needed only when changing the API. Race tests need CGO and a C compiler.

```bash
git clone https://github.com/huey1in/moduapi.git
cd moduapi
go run ./tools/build
```

In terminal 1, start the included mock API. No external credentials are needed:

```bash
go run ./cmd/mock-upstream
```

In terminal 2:

```bash
go run ./cmd/moduapi serve --config config.yaml
```

The compiled command is `./dist/moduapi` on Linux/macOS and `.\dist\moduapi.exe` on Windows. For example:

```bash
./dist/moduapi serve --config config.yaml
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

Send JSON or real streaming SSE:

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"mock","messages":[{"role":"user","content":"Hello"}]}'

curl -N http://127.0.0.1:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"mock","messages":[{"role":"user","content":"Hello"}],"stream":true}'
```

In PowerShell, use `curl.exe` instead of the `curl` alias, or:

```powershell
$body = @{ model = 'mock'; messages = @(@{ role = 'user'; content = 'Hello' }) } | ConvertTo-Json -Depth 5
Invoke-RestMethod http://127.0.0.1:8080/v1/chat/completions -Method Post -ContentType application/json -Body $body
```

Mock models `mock-4xx`, `mock-5xx` and `mock-timeout` exercise failure paths. `examples/roundrobin.yaml` uses two mock servers; start the second with `go run ./cmd/mock-upstream --listen 127.0.0.1:9091`.

## Minimal configuration

```yaml
server:
  listen: "127.0.0.1:8080"
plugins:
  directory: "./dist/plugins"
routes:
  - id: default
    protocol: openai.chat
    model: "*"
    upstreams: [default]
    scheduler: scheduler.single
upstreams:
  default:
    plugin: upstream.http
    config:
      base_url: "http://127.0.0.1:9090/v1"
```

Core owns wiring; `upstreams.*.config` belongs to its upstream plugin and `protocol_config` belongs to the selected protocol plugin. Bundle paths resolve relative to the configuration file. Routes match in order, with exact model names or `*`. The optional `scheduler.priority-weight-health` implements priority, proportional weights, health feedback, capacity leases and soft sticky sessions; the original single/round-robin plugins remain available. See [the scheduler design](docs/design/scheduler.md) and `examples/advanced.yaml`.

For a real OpenAI-compatible service, change the HTTP plugin's `base_url` and set `config.api_key_env: OPENAI_API_KEY`. Add `env:OPENAI_API_KEY` to `permissions.secrets` in **the installed** `dist/plugins/upstream.http/manifest.yaml`, and set that variable in the shell launching the host. Restrict its network permission to the target hostname. Builds overwrite installed demo manifests, so keep deployment bundles separately. Incoming Authorization/Cookie headers are not forwarded as upstream credentials.

There is no ingress authentication plugin in this slice. The example binds to loopback. Access control belongs in a future plugin or an existing authenticated edge.

## Plugin commands

```bash
go run ./cmd/moduapi plugin list --directory dist/plugins
go run ./cmd/moduapi plugin inspect dist/plugins/protocol.openai
go run ./cmd/moduapi plugin validate dist/plugins/upstream.http
```

These commands inspect manifests without running plugin code. `serve` discovers the installed bundles, validates them, starts processes, checks handshake/configuration/health, validates registry references and then serves requests. A failed boot cleans up already-started children. Ready plugins that crash unexpectedly get bounded exponential restart with crash-loop protection. `/readyz` remains unavailable until required plugins recover; `/healthz` reflects kernel liveness.

The v1 API contains Lifecycle, Protocol, Upstream and Scheduler services. The SDK reserves other plugin kind names; those services are not implemented yet. Process isolation **does not provide a security sandbox**. See [the developer guide](docs/plugin-development.md) for permissions, lifecycle and cross-language implementation details.

## Checks

```bash
go test ./...
go test -race ./...
go vet ./...
make conformance # Python 3.11+, isolated pinned virtualenv
# Portable equivalent: go run ./tools/conformance
```

The Phase 3A placement experiment is intentionally separate from ordinary
tests. Cross-compile the audit harness for Linux and follow the reproducible
commands in [the validation report](docs/validation/phase-3a.md); raw results
are kept beside the report. The current decision is [CONDITIONAL GO](docs/validation/phase-3a-exit-gate.md),
pending independent integration requirements and a dedicated-host repeat.

Integration tests build and launch the five actual plugin binaries, use local mock HTTP servers, and instrument child binaries during the race run. They cover streaming before completion, cancellation, deadlines, replay opt-in, failure isolation, negotiation, environment grants, cleanup and restart. Tests do not call OpenAI or Anthropic.

## Design and next work

- [Architecture and lifecycle](docs/architecture.md)
- [Plugin developer guide](docs/plugin-development.md)
- [Formal v1 plugin contract](docs/plugin-contract.md)
- [Advanced scheduler design](docs/design/scheduler.md)
- [Scheduling lessons](docs/research/scheduling-lessons.md)
- [Pinned New API / Sub2API source research](docs/research.md)
- [Architecture decisions](docs/adr/0001-core-boundary.md)
- [Current limitations and next milestones](docs/roadmap.md)
- [Phase 1 validation results](docs/validation.md)
- [Phase 2 validation and benchmark results](docs/validation-phase2.md)
- [Phase 3A composition and placement validation](docs/validation/phase-3a.md)
- [Phase 3A exit gate](docs/validation/phase-3a-exit-gate.md)

Only gRPC, protobuf and YAML parsing are direct dependencies. No code or generated contracts from New API/Sub2API are included. The repository owner still needs to choose ModuAPI's distribution license before a public release.
