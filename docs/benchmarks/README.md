# Scheduler baseline (Phase 2)

Measured locally on 2026-10-16, Windows/amd64, Go 1.27.2, Intel i9-14900HX,
GOMAXPROCS=32, no race instrumentation. [Complete output](windows-amd64.txt) includes
1/10/100/1000 workers at each candidate count. This is a short desktop baseline,
subject to background work, GC and thermal variation, not a throughput/SLA claim.

```bash
go test ./plugins/official/scheduler-priority-weight-health \
  -run '^$' -bench . -benchmem -benchtime=200ms
```

| Operation | Candidates | ns/op | B/op | allocs/op |
|---|---:|---:|---:|---:|
| Select | 10 | 1497 | 1586 | 8 |
| Select | 100 | 12285 | 15945 | 14 |
| Select | 1000 | 141226 | 199254 | 25 |
| Report | 10 | 300.6 | 0 | 0 |
| Report | 100 | 298.0 | 0 | 0 |
| Report | 1000 | 323.0 | 0 | 0 |
| Weighted sampling only | 10 | 71.57 | 0 | 0 |
| Weighted sampling only | 100 | 330.5 | 0 | 0 |
| Weighted sampling only | 1000 | 4077 | 0 | 0 |
| Sticky Select + Report | 10 | 8806 | 2136 | 17 |
| Sticky Select + Report | 100 | 66289 | 16488 | 23 |
| Sticky Select + Report | 1000 | 678377 | 199577 | 34 |

Select batches admission and excludes release time. Report rearms genuine leases
outside timing to isolate feedback from candidate preparation. Parallel and sticky
benchmarks include Select+Report cycles, keeping the live lease table bounded.
Parallel workers are explicit goroutine counts, not `RunParallel` CPU multipliers.
The weighted benchmark isolates the actual sampling helper used by the plugin.

Full Select still parses/scans all candidate hints and builds a tier pool under one
mutex, including on sticky hits. Its linear memory/time growth is visible and is
deliberate technical debt. The measured policies have different timing boundaries;
do not subtract rows to infer sticky or IPC overhead. These tests do not measure
gRPC, kernel HTTP, upstream latency or end-to-end stream backpressure. No optimization
or capacity claim is justified by this short baseline alone.
