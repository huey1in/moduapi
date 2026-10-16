# Python wire reference

Run from repository root: `make conformance` or `go run ./tools/conformance`.
Python 3.11+ and Go are prerequisites. The runner creates `.venv-conformance`,
installs every dependency at a fixed version, regenerates bindings directly from
the public proto, and runs `go test -race -tags=conformance -count=1 ./tests/conformance/...`.
CI runs the same command on Linux and Windows. No global Python package is used.

`plugin.py` implements Lifecycle, Protocol, Upstream and Scheduler using only
protobuf/gRPC Python libraries. Generated `moduapi/plugin/v1` is committed for
inspection and regenerated on each run. There is no Go SDK bridge. Modes and the
event file are test instrumentation, not a production plugin API. Event names
contain no payload or affinity material. Contract: [plugin-contract](../../../docs/plugin-contract.md).
