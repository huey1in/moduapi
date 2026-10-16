# Phase 1 validation

Validated locally on 2026-10-16 using Go 1.27.2, Windows/amd64, CGO enabled and the installed GCC toolchain.

| Command | Result |
|---|---|
| `go test ./...` | PASS; integration package 46.241s in the final run |
| `go test -race ./...` | PASS; integration package 92.753s in the final run |
| `go vet ./...` | PASS |
| `go run ./tools/build` | PASS; host and four process bundles generated |
| Local documentation links and `git diff --check` | PASS |

Integration coverage uses real plugin binaries and local mock HTTP servers, with no external model API calls. Race runs instrument both plugins and the CLI host/mock binaries. Successful process shutdowns are checked for nonzero exit status; owned test bundles are removed with bounded cleanup retries. No test plugin processes remained after the final run.

Covered behaviors include JSON and incrementally delivered SSE, usage, model mapping/opaque fields, single/round-robin/concurrent selection, 4xx/5xx passthrough, opt-in failover and replay budgets, hard commitment guards even against a permissive custom policy, body limits, first-byte/header/body waits, idle/total deadlines, downstream cancellation, truncated streams, protocol terminal completion before socket EOF, active-stream drain, safe headers, connection reuse, redirects, network/env grants, authenticated RPC, strict manifests/versions, discovery, handshake/capability mismatch, failed startup rollback, crashes, explicit restart and CLI commands.

The CI workflow is configured for Windows and Linux. This local validation does not claim that remote CI or Linux execution has already passed. No throughput, allocation or latency benchmark target has been established.
