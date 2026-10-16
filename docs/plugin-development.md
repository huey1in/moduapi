# Plugin developer guide

The source of truth is [`proto/moduapi/plugin/v1/plugin.proto`](../proto/moduapi/plugin/v1/plugin.proto). The Go SDK is in `sdk/`; committed generated Go bindings are in `gen/plugin/v1/`. The five official plugins are complete executable examples. The host does not import their implementations.

## Bundle and manifest

An installed bundle contains `manifest.yaml` and an executable `plugin` (`plugin.exe` on Windows). Discovery recursively finds `manifest.yaml`, skips hidden directories, rejects bundle symlinks and duplicate IDs, and sorts by ID. Maximum 128 manifests; each manifest is at most 256KiB. The host validates unknown/duplicate YAML fields, a single YAML document and required top-level fields before execution.

```yaml
api_version: 1
id: scheduler.example
name: Example Scheduler
version: 0.1.0
runtime:
  type: process
  command: ./plugin
  args: []
provides: [scheduler]
requires:
  gateway: ">=0.1.0 <1.0.0"
capabilities: []
permissions:
  network: []
  storage: []
  secrets: []
config_schema:
  type: object
  additionalProperties: false
```

IDs are lower-case dotted/hyphenated segments, start with a letter, require a separator and are at most 100 characters. Versions are strict three-component SemVer without a `v` prefix; leading numeric zeroes are rejected, including numeric prerelease identifiers. Requirements support a space-separated conjunction of `=`, `<`, `<=`, `>` and `>=` against full versions; there is no caret/tilde/wildcard/OR syntax. Build metadata does not affect comparison. Prerelease versions use SemVer ordering with no implicit npm-style exclusion.

The command is package-relative and executed directly, never through a shell. Traversal, absolute paths and resolved binary escapes are rejected. There are at most 64 runtime arguments. The host appends `.exe` for an extensionless command on Windows. One plugin version per ID is loaded in a boot.

`config_schema` is a JSON-compatible object-schema declaration for discovery/tooling. V1 checks its root shape, **not a full JSON Schema vocabulary**. The plugin must perform semantic configuration validation in `Configure`. Official plugins currently accept only an empty process configuration. Their HTTP and protocol per-upstream configuration is validated by those plugins on request, not by Core at startup. Unknown Core fields are rejected; opaque plugin configuration fields belong to the plugin.

## Implementing a Go plugin

A minimal first-candidate scheduler:

```go
package main

import (
    "context"
    "fmt"
    "os"
    "slices"
    "github.com/huey1in/moduapi/sdk"
    "github.com/huey1in/moduapi/sdk/plugin"
)

type scheduler struct{}

func (scheduler) Select(ctx context.Context, in *sdk.SelectionInput) (*sdk.Selection, error) {
    if err := ctx.Err(); err != nil { return nil, err }
    for _, c := range in.Candidates {
        if !slices.Contains(in.ExcludedIds, c.Id) {
            return &sdk.Selection{CandidateId: c.Id, Reason: "first eligible"}, nil
        }
    }
    return nil, sdk.Fail(sdk.Local, "no_candidate", "no eligible upstream", 503)
}

func (scheduler) Report(ctx context.Context, result *sdk.AttemptResult) error {
    return ctx.Err()
}

func main() {
    err := plugin.Run(plugin.Definition{
        Info: &sdk.Description{
            Id: "scheduler.example", Version: "0.1.0",
            Provides: []string{sdk.KindScheduler},
        },
        Scheduler: scheduler{},
    })
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
}
```

Compile into a bundle directory with a matching manifest; copy it under the configured plugin directory. `plugin validate` is static validation. `serve` validates the actual negotiated identity and health. The runtime only activates the three implemented v1 kinds: `protocol`, `upstream`, `scheduler`. Provider/authenticator/middleware/storage/usage/billing/quota/observability/admin/secret-provider names are reserved; they have no active process service yet. Experimental Go extension sketches in `sdk/extensions.go` are not a compatibility promise for future wire APIs.

## Bootstrap for any language

1. Read `MODUAPI_PLUGIN_TOKEN`. Bind a gRPC server to `127.0.0.1:0`.
2. Require RPC metadata `authorization: Bearer <token>` on every unary and streaming call.
3. Print exactly one JSON line to stdout, shorter than 4096 bytes:

   ```json
   {"api_version":1,"address":"127.0.0.1:12345"}
   ```

4. Reserve stdout thereafter. Use stderr for local debugging; the current host discards plugin stderr. Never log payloads or credentials. A structured, bounded diagnostic sink is future work.
5. Implement `Lifecycle.Handshake`, `Configure`, `Health`, `Shutdown` and the declared service(s).
6. Observe stdin EOF as parent death. Stop outstanding RPCs and exit. Do not create unmanaged descendants.

The host sends API version, gateway version and offered capabilities. Return your exact manifest identity, version, provides and supported negotiated capabilities. Every declared capability must be supported by both sides; unknown mandatory capabilities fail startup. V1 offers `streaming`, `usage` and `affinity`; affinity is an opaque hint consumed by the optional priority-weight-health scheduler. Protocol descriptions declare method/path/protocol endpoints. Endpoint collisions and declarations from non-protocol plugins fail boot.

`Configure` receives an opaque JSON object once. Return an error to reject it. `Health` returns readiness after configure. The host polls every five seconds. A shutdown RPC starts draining, acknowledges, waits for existing calls and exits; the host kills/reaps after two seconds. On failed startup there is no drain period. `Manager.Restart` is explicit and requires unchanged description/bindings. Unexpected ready-process exits now receive bounded automatic recovery with crash-loop protection. There is no operator restart CLI/control API yet.

Generate client/server bindings with the language's official protobuf/gRPC tools. Rust/Node/Python can implement this protocol without linking Go. The Go SDK/examples and an independent Python reference/conformance suite are delivered; Rust/Node references remain future work. Do not rename/reuse existing field numbers; additive optional fields keep v1 compatible, incompatible changes need a new package/API version.

For Go regeneration, use `protoc 36.2` (the release binary reports `libprotoc 36.2`) and:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1
make generate
```

Place the binaries on PATH. The normal build consumes committed generated files.

## Service responsibilities

| Service | Calls | Responsibility |
|---|---|---|
| Lifecycle | Handshake, Configure, Health, Shutdown | Compatibility and process lifecycle |
| Protocol | Decode, Prepare, Transform | Ingress validation, canonical payload, per-attempt mapping, response conversion and usage |
| Upstream | Execute | Cancellable transport with streamed response frames |
| Scheduler | Select, Report | Selection, optional affinity/reservations, feedback and release |

`Transform` is bidirectional: its first input is the canonical request; subsequent inputs are upstream frames. Output is the downstream frame sequence. Keep conversion/usage state local to this RPC; never store it in global request maps. The host forwards input concurrently with receiving output so flow control does not deadlock. Always honor context on both directions.

`Select` gets a small `RequestInfo`, not body bytes or upstream configuration/credentials. Candidates have IDs, opaque metadata and host-local in-flight counts. `excluded_ids` must override sticky preferences. The host validates the chosen ID. `lease` is opaque feedback state and is returned by `Report` even for cancelled attempts or hook rejection. Reporting is best effort, with a bounded cleanup context; use idempotent releases and expiring leases. A selection call cancelled after an external reservation needs a plugin-owned recovery path too.

Failures distinguish local rejection, upstream HTTP, transport, protocol, downstream cancellation, timeout and plugin errors. Unary errors use a protobuf `Failure` status detail. Streaming failures are terminal frames. Public messages must omit addresses, keys and request content. `request_sent` is conservative diagnostic evidence and is not proof of idempotency. Downstream cancellation does not warrant retry or health penalties.

## Frame and stream contract

- Exactly one start with HTTP status 200–599, followed by data chunks of 0..32KiB, then exactly one end. A failure may terminate before or after start. EOF without a terminal is failure.
- RPC messages are limited to 10MiB; ingress bodies are separately capped at at most 8MiB. Header/metadata overhead counts toward the RPC cap.
- `Send` returning success means the bytes were accepted by the RPC stack. Do not mutate a frame concurrently with Send. Reusing a buffer after synchronous Send returns is supported by the Go implementation.
- `end.usage` or `failure.usage` carries observed tokens; failures and disconnects can be partial. Core emits a lifecycle usage event but does not price, charge, persist or promise durable delivery.
- Do not turn an incomplete stream into an end/success. Protocol-specific termination (for example `[DONE]`) belongs to Protocol, not the kernel.
- Consumer code must call `Stream.Close` on every path. It cancels upstream RPCs and joins the host sender. Never detach a transport from the supplied context to continue billing after a client leaves.
- Slow consumers apply backpressure through bounded gRPC windows; do not introduce unbounded queues. The kernel separately bounds reads, writes and total duration.

## Official configuration

`upstream.http` per-upstream config:

```yaml
plugin: upstream.http
config:
  base_url: https://api.example.com/v1
  api_key_env: EXAMPLE_API_KEY
  headers:
    X-Deployment: primary
protocol_config:
  model_mapping:
    public-model: upstream-model
metadata:
  moduapi.scheduler.priority: "10"
  moduapi.scheduler.weight: "3"
```

The HTTP plugin owns `base_url`, `api_key_env` and `headers`. The OpenAI protocol owns `model_mapping`. The base URL must be HTTP(S), without userinfo/query/fragment; redirects are not followed. Network globs are hostnames, without port or scheme; `*.example.com` matches subdomains, not the apex. The demo grants `*` to the HTTP plugin so localhost and arbitrary compatible deployments work. Restrict deployment grants explicitly. No ambient proxy environment is inherited or used. All configured upstreams share this process's transport limits.

## Permission/security contract

**Process isolation != security sandbox.** Only run plugins you trust.

The host inherits only basic OS executable/temp variables (`SystemRoot`, `WINDIR`, `PATH`, `TEMP`, `TMP`), explicitly granted `env:NAME` secret variables, and the two runtime variables for the token and network declaration. Host credentials such as GitHub tokens are not inherited by default. A secret declaration beginning `env:MODUAPI_` is forbidden. Granted environment variables are readable by the whole plugin process; they are not request-scoped secret handles.

`permissions.network` is passed as JSON in `MODUAPI_NETWORK_PERMISSIONS`; the official HTTP plugin checks the configured target hostname against it. This check is cooperative. The host does not intercept arbitrary plugin sockets, filesystem calls or subprocesses. Storage permission namespaces are declared/validated only; no storage broker exists. There is no filesystem/network namespace, seccomp, job object, container, resource cgroup, mTLS, package signing or binary checksum verification in this phase. Code running under the host OS user can access that user's resources despite the manifest.

Do not advertise these declarations as enforcement for untrusted binaries. Stronger enforcement and scoped host secret/storage brokers require separate ADRs and platform work. Current limits and priorities are in [the roadmap](roadmap.md).

## Phase 2 contract and reference implementation

The normative [v1 contract](plugin-contract.md) supplements this guide: terminal
frames require actual RPC EOF, empty data frames are legal, and status fallbacks
preserve cancellation/timeout/local/plugin distinctions. No protobuf service/field
was changed. The independent [Python reference](../tests/conformance/python/README.md)
and `make conformance` exercise all services without a Go SDK dependency.

Interpreter example:

```yaml
runtime:
  type: interpreter
  interpreter: /opt/moduapi/python-venv/bin/python
  command: plugin.py
  args: []
```

A bare interpreter name resolves through PATH; scripts remain inside their package.
Native command containment remains unchanged. No shell or interpreter flags are
embedded in the interpreter name. Pin the interpreter environment in deployments.

The [advanced scheduler](design/scheduler.md) owns all policy/state and recognizes
only namespaced generic hints. `X-Moduapi-Affinity` is an optional OpenAI protocol
ingress hint, not an identity credential, and is never forwarded upstream.

Runtime embedders may use `NewManagerWithOptions(RestartOptions)` and
`Manager.Diagnostics(id)` for ready/failed/PID/uptime/restarts/safe stderr tail.
CLI inspect remains offline manifest inspection; there is no daemon control socket.
The 64KiB stderr tail accepts only JSON `{event, code?}` with events startup, ready,
draining, stopped, failed and codes config_rejected, transport_error, protocol_error,
cancelled, timeout, internal_error, crash (or omitted). Other fields/text are dropped.
Never put sensitive material in any log channel.
