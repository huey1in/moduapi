# Embedding Host: validation brief

This is an unsent validation template, not an adopter report. Baseline `dfde6e6`,
2026-10-17. We want a Go service owner to test whether `http.Handler` composition
plus managed replaceable components is preferable to Bifrost core, an existing
proxy/processor deployment, or a small client/router library.

## What the host receives and still owns

`embedding.Start` starts the component set; `Ready` is a status snapshot;
`ServeHTTP` composes the request lifecycle; `Close` cancels/stops/reaps. The
host owns HTTP serving/TLS/drain, edge authentication/admission, secrets/config
delivery, deployment, observability and application tests. Start's context only
bounds startup. Close is not a hard deadline and cancels active requests; drain
the HTTP server first for graceful completion. Read the
[API review](embedding-api-review.md) before treating four methods as a small
stable contract: configuration still exposes `core/config` and `core/router`.

## Runnable separate-module example

The following is documentation-only reproduction code. It creates two loopback
HTTP listeners and sends a real request through three installed process
components. It imports **no** ModuAPI `internal` helper or reference CLI. It is
our own validation, not an independent user's demand. Use Go 1.27.2 and build
the existing trusted components first. On Linux/macOS with a Bash shell:

```bash
# Run from the ModuAPI checkout. Do not put a trial module inside that module.
repo="$(pwd)"
go run ./tools/build
trial="$(mktemp -d)"
mkdir -p "$trial/plugins"
for id in protocol.openai scheduler.single upstream.http; do
  cp -R "$repo/dist/plugins/$id" "$trial/plugins/$id"
done
cd "$trial"
go mod init example.com/moduapi-demand-check
go mod edit -replace "github.com/huey1in/moduapi=$repo"
go mod edit -require github.com/huey1in/moduapi@v0.0.0
```

Save the following as `main.go` in that trial directory. The local `replace`
pins the checked-out implementation; it is not a released dependency or license
grant. Only the three needed bundles are installed because all discovered
bundles start and affect readiness.

```go
package main

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "strings"
    "time"

    "github.com/huey1in/moduapi/core/config"
    "github.com/huey1in/moduapi/core/router"
    "github.com/huey1in/moduapi/embedding"
)

func run() error {
    if len(os.Args) != 2 {
        return fmt.Errorf("usage: demand-check ABSOLUTE_PLUGIN_DIRECTORY")
    }
    mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/v1/chat/completions" {
            http.NotFound(w, r)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        io.WriteString(w, `{"id":"mock","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
    }))
    defer mock.Close()
    c := config.Config{
        Plugins: config.Plugins{Directory: os.Args[1]},
        Routes: []router.Route{{ID: "trial", Protocol: "openai.chat", Model: "*",
            Scheduler: "scheduler.single", Upstreams: []string{"mock"}}},
        Upstreams: map[string]config.Upstream{"mock": {
            Plugin: "upstream.http", Config: config.Object{"base_url": mock.URL + "/v1"},
        }},
    }
    startup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    rt, err := embedding.Start(startup, c)
    cancel()
    if err != nil {
        return err
    }
    defer func() {
        ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
        defer stop()
        rt.Close(ctx)
    }()
    gateway := httptest.NewServer(rt)
    defer gateway.Close()
    client := &http.Client{Timeout: 5*time.Second}
    req, err := http.NewRequest(http.MethodPost, gateway.URL+"/v1/chat/completions",
        strings.NewReader(`{"model":"mock","messages":[{"role":"user","content":"hello"}]}`))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
    resp.Body.Close()
    if err != nil {
        return err
    }
    if resp.StatusCode != 200 || !strings.Contains(string(data), `"content":"ok"`) {
        return fmt.Errorf("unexpected mock response: status=%d", resp.StatusCode)
    }
    fmt.Printf("ready=%t status=%d\n", rt.Ready(), resp.StatusCode)
    return nil
}

func main() {
    if err := run(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

Then run:

```bash
go mod tidy
go run . "$trial/plugins"
```

Expected: `ready=true status=200`, followed by normal component cleanup. This
short smoke example has no concurrent work at shutdown. A real host should
first call its `http.Server.Shutdown`, then `rt.Close`, with its own abort policy.
The Go block was executed from a temporary external module on Windows/Go 1.27.2
during this review (equivalent path setup); that is not native Linux validation.
The existing repository boundary regression can also be run with:

```bash
go test -count=1 ./embedding
```

## Questions for the actual host owner

1. Is this an HTTP gateway requirement or a client-library requirement? Why
   not Bifrost core or the current proxy plus an extension?
2. Which protocol/selection/transport components must be independently
   replaced, in what languages, and by which release owners?
3. Which `core/config`/`core/router` types would become public dependencies of
   your service? What upgrades or shutdown behavior would be unacceptable?
4. Can you install just the required trusted bundles? Who operates interpreter
   environments, crash recovery, logs and mixed-version compatibility?
5. What existing listener, admission, configuration and security work would
   remain? Is a second process boundary an operational benefit or burden?
6. What measured latency/TTFB/body-size/CPU/RSS limits apply? Would bundle or
   the best existing in-process alternative win for your actual traffic?
7. Can you provide a participant-owned integration commit and same-scope
   alternative, then review saved and added work using the
   [cost record](integration-cost-record.md)? What would make you reject it?

No affirmative answer or cost entry has been supplied by an independent host.
Do not use this example, its line count, or passing CI to discharge that gate.
