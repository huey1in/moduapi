package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huey1in/moduapi/internal/buildutil"
)

// Exercise the documented binaries and configuration loader, in addition to
// the direct handler integration tests. Every child is owned and reaped.
func TestCLI(t *testing.T) {
	root, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	build := func(name, pkg string) string {
		t.Helper()
		path := filepath.Join(dir, name+buildutil.Suffix())
		args := []string{"build"}
		if raceEnabled {
			args = append(args, "-race")
		}
		args = append(args, "-o", path, pkg)
		cmd := exec.Command("go", args...)
		cmd.Dir = root
		if output, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("%v: %s", e, output)
		}
		return path
	}
	host := build("moduapi", "./cmd/moduapi")
	mock := build("mock-upstream", "./cmd/mock-upstream")
	for _, args := range [][]string{{"version"}, {"plugin", "list", "--directory", bundles}, {"plugin", "validate", filepath.Join(bundles, "protocol.openai")}, {"plugin", "inspect", filepath.Join(bundles, "upstream.http")}} {
		cmd := exec.Command(host, args...)
		b, e := cmd.CombinedOutput()
		if e != nil || len(b) == 0 {
			t.Fatalf("%v: %s %v", args, b, e)
		}
	}
	freeAddress := func() string {
		t.Helper()
		lis, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		addr := lis.Addr().String()
		lis.Close()
		return addr
	}
	upAddr, hostAddr := freeAddress(), freeAddress()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	start := func(binary string, args ...string) {
		t.Helper()
		childCtx, stop := context.WithCancel(ctx)
		cmd := exec.CommandContext(childCtx, binary, args...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if e := cmd.Start(); e != nil {
			stop()
			t.Fatal(e)
		}
		t.Cleanup(func() { stop(); _ = cmd.Wait() })
	}
	start(mock, "--listen", upAddr)
	data, _ := json.Marshal(map[string]any{"server": map[string]any{"listen": hostAddr}, "plugins": map[string]any{"directory": bundles}, "routes": []any{map[string]any{"id": "cli", "protocol": "openai.chat", "model": "*", "upstreams": []string{"default"}, "scheduler": "scheduler.single"}}, "upstreams": map[string]any{"default": map[string]any{"plugin": "upstream.http", "config": map[string]any{"base_url": "http://" + upAddr + "/v1"}}}})
	path := filepath.Join(dir, "config.yaml")
	if e = os.WriteFile(path, data, 0644); e != nil {
		t.Fatal(e)
	}
	start(host, "serve", "--config", path)
	client := &http.Client{Timeout: time.Second}
	ready := false
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); {
		r, e := client.Get("http://" + hostAddr + "/readyz")
		if e == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				ready = true
				break
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
	if !ready {
		t.Fatal("CLI readiness timed out")
	}
	for _, stream := range []bool{false, true} {
		b, _ := json.Marshal(map[string]any{"model": "mock", "messages": []any{map[string]string{"role": "user", "content": "hello"}}, "stream": stream})
		r, e := client.Post("http://"+hostAddr+"/v1/chat/completions", "application/json", bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		output, e := io.ReadAll(r.Body)
		r.Body.Close()
		if e != nil || r.StatusCode != 200 {
			t.Fatalf("CLI response: %d %s %v", r.StatusCode, output, e)
		}
		want := "Hello from ModuAPI mock"
		if stream {
			want = "[DONE]"
		}
		if !strings.Contains(string(output), want) {
			t.Fatalf("CLI response missing %s: %s", want, output)
		}
	}
}
