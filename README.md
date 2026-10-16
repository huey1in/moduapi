# ModuAPI

**A lightweight, plugin-first AI API gateway.**

ModuAPI keeps the HTTP runtime small and runs protocol, upstream and scheduling implementations as independent processes. Install only the capabilities you need. The kernel starts without a database, Redis, users, authentication management, billing or an admin UI.

This repository contains the first runnable vertical slice: OpenAI Chat Completions → router → scheduler → generic HTTP upstream → JSON/SSE. It is an initial implementation, not a production hardening claim.

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

Core owns wiring; `upstreams.*.config` belongs to its upstream plugin and `protocol_config` belongs to the selected protocol plugin. Bundle paths resolve relative to the configuration file. Routes match in order, with exact model names or `*`. Priority/weight metadata is available to scheduler plugins; the included single/round-robin schedulers do not implement weighting.

For a real OpenAI-compatible service, change the HTTP plugin's `base_url` and set `config.api_key_env: OPENAI_API_KEY`. Add `env:OPENAI_API_KEY` to `permissions.secrets` in **the installed** `dist/plugins/upstream.http/manifest.yaml`, and set that variable in the shell launching the host. Restrict its network permission to the target hostname. Builds overwrite installed demo manifests, so keep deployment bundles separately. Incoming Authorization/Cookie headers are not forwarded as upstream credentials.

There is no ingress authentication plugin in this slice. The example binds to loopback. Access control belongs in a future plugin or an existing authenticated edge.

## Plugin commands

```bash
go run ./cmd/moduapi plugin list --directory dist/plugins
go run ./cmd/moduapi plugin inspect dist/plugins/protocol.openai
go run ./cmd/moduapi plugin validate dist/plugins/upstream.http
```

These commands inspect manifests without running plugin code. `serve` discovers the installed bundles, validates them, starts processes, checks handshake/configuration/health, validates registry references and then serves requests. A failed boot cleans up already-started children.

The v1 API contains Lifecycle, Protocol, Upstream and Scheduler services. The SDK reserves other plugin kind names; those services are not implemented yet. Process isolation **does not provide a security sandbox**. See [the developer guide](docs/plugin-development.md) for permissions, lifecycle and cross-language implementation details.

## Checks

```bash
go test ./...
go test -race ./...
go vet ./...
```

Integration tests build and launch the four actual plugin binaries, use local mock HTTP servers, and instrument child binaries during the race run. They cover streaming before completion, cancellation, deadlines, replay opt-in, failure isolation, negotiation, environment grants, cleanup and restart. Tests do not call OpenAI or Anthropic.

## Design and next work

- [Architecture and lifecycle](docs/architecture.md)
- [Plugin developer guide and wire contract](docs/plugin-development.md)
- [Pinned New API / Sub2API source research](docs/research.md)
- [Architecture decisions](docs/adr/0001-core-boundary.md)
- [Current limitations and next milestones](docs/roadmap.md)
- [Phase 1 validation results](docs/validation.md)

Only gRPC, protobuf and YAML parsing are direct dependencies. No code or generated contracts from New API/Sub2API are included. The repository owner still needs to choose ModuAPI's distribution license before a public release.
