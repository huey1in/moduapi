package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/huey1in/moduapi/gen/plugin/v1"
	openai "github.com/huey1in/moduapi/plugins/official/protocol-openai"
	single "github.com/huey1in/moduapi/plugins/official/scheduler-single"
	"github.com/huey1in/moduapi/runtime/manifest"
	"github.com/huey1in/moduapi/runtime/process"
	"github.com/huey1in/moduapi/runtime/registry"
	"github.com/huey1in/moduapi/sdk"
	"github.com/huey1in/moduapi/sdk/plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type helperScheduler struct{ single.Scheduler }

func (helperScheduler) Select(ctx context.Context, r *sdk.SelectionInput) (*sdk.Selection, error) {
	return &sdk.Selection{CandidateId: "a", Reason: os.Getenv("TEST_PLUGIN_ALLOWED") + "|" + os.Getenv("TEST_PLUGIN_DENIED")}, ctx.Err()
}
func runHelper() {
	mode := "serve"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch mode {
	case "crash":
		os.Exit(23)
	case "stall":
		time.Sleep(30 * time.Second)
		return
	case "badbootstrap":
		fmt.Println(`{"api_version":1,"address":"0.0.0.0:1234"}`)
		time.Sleep(30 * time.Second)
		return
	}
	d := plugin.Definition{Info: &sdk.Description{Id: "test.helper", Version: "0.1.0", Provides: []string{"scheduler"}}, Scheduler: helperScheduler{}}
	if mode == "reject-config" {
		d.Configure = func(context.Context, json.RawMessage) error { return errors.New("rejected") }
	}
	if mode == "duplicate-endpoint" {
		d.Protocol = openai.Protocol{}
		d.Info.Provides = append(d.Info.Provides, "protocol")
		d.Info.Endpoints = []*sdk.Endpoint{{Method: "POST", Path: "/example", Protocol: "test.protocol"}, {Method: "POST", Path: "/example", Protocol: "test.protocol"}}
	}
	if e := plugin.Run(d); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func helperManifest(t *testing.T, mode string) manifest.Manifest {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	name := "plugin" + filepath.Ext(exe)
	if e = os.WriteFile(filepath.Join(dir, name), data, 0755); e != nil {
		t.Fatal(e)
	}
	return manifest.Manifest{ID: "test.helper", Name: "Test Helper", Version: "0.1.0", APIVersion: 1, Runtime: manifest.Runtime{Type: "process", Command: "./" + name, Args: []string{mode}}, Provides: []string{"scheduler"}, Requires: manifest.Requirements{Gateway: ">=0.1.0 <1.0.0"}, ConfigSchema: map[string]any{"type": "object"}, Path: filepath.Join(dir, "manifest.yaml")}
}
func TestPluginRuntime(t *testing.T) {
	t.Run("identity version capability negotiation", func(t *testing.T) {
		m, e := manifest.Read(filepath.Join(bundles, "protocol.openai", "manifest.yaml"))
		if e != nil {
			t.Fatal(e)
		}
		for _, tc := range []struct {
			name   string
			change func(*manifest.Manifest)
		}{{"identity", func(m *manifest.Manifest) { m.ID = "protocol.other" }}, {"version", func(m *manifest.Manifest) { m.Version = "0.2.0" }}, {"capability", func(m *manifest.Manifest) { m.Capabilities = append(m.Capabilities, "future.capability") }}} {
			t.Run(tc.name, func(t *testing.T) {
				decl := m
				decl.Capabilities = append([]string(nil), m.Capabilities...)
				tc.change(&decl)
				p, e := process.Start(context.Background(), decl, nil)
				if e == nil {
					p.Stop(context.Background())
					t.Fatal("mismatch accepted")
				}
				if !strings.Contains(e.Error(), "mismatch") {
					t.Fatal(e)
				}
			})
		}
	})
	t.Run("startup failures cleaned up", func(t *testing.T) {
		for _, mode := range []string{"crash", "stall", "badbootstrap", "reject-config"} {
			t.Run(mode, func(t *testing.T) {
				m := helperManifest(t, mode)
				limit := 5 * time.Second
				if mode == "stall" {
					limit = 300 * time.Millisecond
				}
				ctx, cancel := context.WithTimeout(context.Background(), limit)
				defer cancel()
				started := time.Now()
				p, e := process.Start(ctx, m, nil)
				if e == nil {
					p.Stop(context.Background())
					t.Fatal("bad startup accepted")
				}
				if time.Since(started) > 7*time.Second {
					t.Fatal("startup cleanup exceeded bound")
				}
				if mode == "reject-config" && !strings.Contains(e.Error(), "configure") {
					t.Fatalf("configuration rejection was not reached: %v", e)
				}
				if mode == "badbootstrap" && !strings.Contains(e.Error(), "invalid plugin bootstrap") {
					t.Fatalf("bootstrap validation was not reached: %v", e)
				}
			})
		}
	})
	t.Run("environment is explicit and RPC requires token", func(t *testing.T) {
		t.Setenv("TEST_PLUGIN_ALLOWED", "explicit")
		t.Setenv("TEST_PLUGIN_DENIED", "host-secret")
		m := helperManifest(t, "serve")
		m.Permissions.Secrets = []string{"env:TEST_PLUGIN_ALLOWED"}
		p, e := process.Start(context.Background(), m, nil)
		if e != nil {
			t.Fatal(e)
		}
		defer p.Stop(context.Background())
		v, e := pb.NewSchedulerClient(p.Conn).Select(context.Background(), &pb.SelectionInput{})
		if e != nil || v.Reason != "explicit|" {
			t.Fatalf("unexpected environment %v %v", v, e)
		}
		// The same endpoint without the per-process token cannot call lifecycle RPCs.
		conn, e := grpc.NewClient(p.Conn.Target(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithNoProxy())
		if e != nil {
			t.Fatal(e)
		}
		defer conn.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, e = pb.NewLifecycleClient(conn).Health(ctx, &pb.Empty{}); e == nil {
			t.Fatal("unauthenticated local RPC allowed")
		}
	})
	t.Run("registry rejects conflicting protocol bindings", func(t *testing.T) {
		m := helperManifest(t, "duplicate-endpoint")
		m.Provides = append(m.Provides, "protocol")
		manager := process.NewManager()
		defer manager.Close(context.Background())
		if e := manager.Start(context.Background(), []manifest.Manifest{m}, nil); e != nil {
			t.Fatal(e)
		}
		if _, e := registry.New(manager); e == nil {
			t.Fatal("duplicate endpoint accepted")
		}
	})
	t.Run("crash isolation restart and graceful shutdown", func(t *testing.T) {
		m := helperManifest(t, "serve")
		manager := process.NewManager()
		defer manager.Close(context.Background())
		if e := manager.Start(context.Background(), []manifest.Manifest{m}, nil); e != nil {
			t.Fatal(e)
		}
		p, e := manager.Get(m.ID)
		if e != nil {
			t.Fatal(e)
		}
		pid := p.PID()
		child, e := os.FindProcess(pid)
		if e != nil {
			t.Fatal(e)
		}
		if e = child.Kill(); e != nil {
			t.Fatal(e)
		}
		wait(t, p.Done(), "crashed process was not reaped")
		if _, e = manager.Get(m.ID); e == nil {
			t.Fatal("crashed process still ready")
		}
		if e = manager.Restart(context.Background(), m.ID); e != nil {
			t.Fatal(e)
		}
		next, e := manager.Get(m.ID)
		if e != nil || next.PID() == pid {
			t.Fatalf("restart failed %v", e)
		}
		next.Stop(context.Background())
		next.Stop(context.Background())
		wait(t, next.Done(), "graceful stop not reaped")
		if e = next.ExitError(); e != nil {
			t.Fatalf("unclean graceful exit: %v", e)
		}
	})
	t.Run("failed boot rolls back earlier processes", func(t *testing.T) {
		good := helperManifest(t, "serve")
		bad, e := manifest.Read(filepath.Join(bundles, "protocol.openai", "manifest.yaml"))
		if e != nil {
			t.Fatal(e)
		}
		bad.Version = "0.2.0"
		manager := process.NewManager()
		defer manager.Close(context.Background())
		if e = manager.Start(context.Background(), []manifest.Manifest{good, bad}, nil); e == nil {
			t.Fatal("mismatch accepted")
		}
		for _, p := range manager.List() {
			wait(t, p.Done(), "startup rollback leaked process")
			if p.Ready() {
				t.Fatal("rollback left ready process")
			}
		}
	})
}
