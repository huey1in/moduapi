# External Component Author: validation brief

This is an invitation **template**, not a sent invitation or an adoption claim.
Baseline `dfde6e6`, 2026-10-17. We want to learn whether a real component author
needs independent protocol, transport or selection releases across languages.
If an existing gateway's configuration/native hook solves the task, that is a
valid rejection of ModuAPI for the task. No production readiness promise is made.

## Boundary under test

The existing [v1 contract](../../proto/moduapi/plugin/v1/plugin.proto) has four
Lifecycle methods plus Protocol (`Decode`, `Prepare`, bidi `Transform`),
Upstream (`Execute` stream) and Scheduler (`Select`, `Report`). A package may
provide one or several typed services. Native executables or interpreter scripts
start through a validated manifest; body semantics and policy state remain in
the component. Kernel owns cancellation, timeout, retry safety, stream ownership
and process supervision. Use the [contract](../plugin-contract.md) for exact
limits, error attribution and terminal-frame rules.

The Python [Messages reference](../../plugins/reference/protocol-anthropic/README.md)
demonstrates the minimal protocol subset; it is **not** full Anthropic/provider
support. It imports generated protobuf/gRPC bindings, not Go SDK code. The
manifest, interpreter environment, configure/health/drain server, stream parser
and upgrade tests are still author work. This is trusted operator-installed
code, not a sandbox or marketplace execution model.

## Runnable example: real foreign-language process, no paid API

Prerequisites: Go 1.27.2, Python 3.11+, a C toolchain supported by Go's race
detector, dependency-download access. From this checkout:

```bash
go run ./tools/conformance
```

This is the portable equivalent of `make conformance`. It creates
`.venv-conformance`, installs the exact pinned
[requirements](../../tests/conformance/python/requirements.txt), regenerates
bindings and runs race-enabled tests with the conformance tag. It includes
[`TestIndependentMessagesProcessAndGateway`](../../tests/conformance/anthropic/anthropic_test.go),
which starts the Python protocol via the real Runtime alongside the Go generic
HTTP upstream and scheduler, sends JSON/SSE against a local mock, preserves an
unknown JSON field, and shuts down. The [wire suite](../../tests/conformance/contract_test.go)
also exercises version/capability errors, unknown protobuf fields, invalid
stream sequences, cancellation and shutdown. A passing run is composition
evidence, **not** a requirement or integration-saving measurement.

For a participant trial, retain the existing service kind and substitute their
implementation/package in a **private test directory**. Record: service methods
implemented, generated vs handwritten files, manifest/config edits, protocol
tests, language-runtime setup and release/update procedure. No new Host service
or kind should be added to make the trial pass. Unsupported callbacks are a
limitation to report, not permission to expand Kernel scope.

## Questions before any implementation

1. Which component do you maintain now, and what fails with the best existing
   hook/configuration? Provide a reproducible input/output or selection rule.
2. Do you require a different language, separate release, or separate crash
   domain? Which is mandatory, and why is an in-process hook insufficient?
3. Do all three components need substitution, or only one? What other component
   must be independently replaced without your release changing?
4. What host callbacks/data do you require? Would their absence make this runtime
   unusable? Can they stay in your component/service instead?
5. Which solution would you choose without ModuAPI: Bifrost, APISIX, Envoy,
   LiteLLM, EPP or go-plugin? What work already exists and must not be recounted?
6. What body sizes/QPS/TTFB/resource budget are real? Is the current
   [external-process cost](phase-3a-workload-fit.md) unacceptable?
7. Who would own packaging, compatibility tests and on-call recovery for the
   next year? What evidence would make you decline the integration?

Use the [cost record](integration-cost-record.md). Agree tests first, then
record both the best alternative and ModuAPI, including negative results.
The 292-line reference implementation is not a saved-code claim. Only the
participant can confirm that the task matters and the benefit offsets runtime
and maintenance costs. No automatic contact or publication is authorized.
