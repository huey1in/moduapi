// Package process owns plugin child lifetimes, never routing or business state.
package process

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/huey1in/moduapi/gen/plugin/v1"
	"github.com/huey1in/moduapi/runtime/manifest"
	"github.com/huey1in/moduapi/sdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Process struct {
	Manifest    manifest.Manifest
	Info        *pb.Description
	Conn        *grpc.ClientConn
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	done        chan struct{}
	monitorDone chan struct{}
	stopMonitor context.CancelFunc
	ready       atomic.Bool
	stopping    atomic.Bool
	stopOnce    sync.Once
	StartedAt   time.Time
	stopped     chan struct{}
	waitErr     error
	stderr      *stderrTail
}

func (p *Process) Ready() bool {
	select {
	case <-p.done:
		return false
	default:
		return !p.stopping.Load() && p.ready.Load()
	}
}
func (p *Process) PID() int              { return p.cmd.Process.Pid }
func (p *Process) Done() <-chan struct{} { return p.done }

// CloseInput signals parent EOF; primarily useful to embedders shutting down
// after their control connection has failed. Stop still owns reaping/cleanup.
func (p *Process) CloseInput() error { return p.stdin.Close() }

// ExitError is meaningful after Done closes; an intentional kill returns an error.
func (p *Process) ExitError() error {
	select {
	case <-p.done:
		return p.waitErr
	default:
		return nil
	}
}
func executable(m manifest.Manifest) (string, error) {
	base := filepath.Dir(m.Path)
	path := filepath.Join(base, m.Runtime.Command)
	if m.Runtime.Type == "process" && runtime.GOOS == "windows" && filepath.Ext(path) == "" {
		path += ".exe"
	}
	resolved, e := filepath.EvalSymlinks(path)
	if e != nil {
		return "", e
	}
	root, e := filepath.EvalSymlinks(base)
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(root, resolved)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("binary escapes plugin package")
	}
	return resolved, nil
}
func Start(ctx context.Context, m manifest.Manifest, config json.RawMessage) (*Process, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if e := m.Validate(); e != nil {
		return nil, e
	}
	path, e := executable(m)
	if e != nil {
		return nil, e
	}
	if len(config) == 0 {
		config = []byte("{}")
	}
	var obj map[string]any
	if json.Unmarshal(config, &obj) != nil || obj == nil {
		return nil, fmt.Errorf("plugin config must be a JSON object")
	}
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	secret := make([]byte, 32)
	if _, e = rand.Read(secret); e != nil {
		return nil, e
	}
	token := hex.EncodeToString(secret)
	args := slices.Clone(m.Runtime.Args)
	if m.Runtime.Type == "interpreter" {
		interpreter, err := exec.LookPath(m.Runtime.Interpreter)
		if err != nil {
			return nil, fmt.Errorf("resolve interpreter: %w", err)
		}
		args = append([]string{path}, args...)
		path = interpreter
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = filepath.Dir(m.Path)
	for _, k := range []string{"SystemRoot", "WINDIR", "PATH", "TEMP", "TMP"} {
		if v, ok := os.LookupEnv(k); ok {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	for _, permission := range m.Permissions.Secrets {
		k := strings.TrimPrefix(permission, "env:")
		if v, ok := os.LookupEnv(k); ok {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	networkJSON, _ := json.Marshal(m.Permissions.Network)
	cmd.Env = append(cmd.Env, "MODUAPI_PLUGIN_TOKEN="+token, "MODUAPI_NETWORK_PERMISSIONS="+string(networkJSON))
	tail := newStderrTail()
	cmd.Stderr = tail
	stdin, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		stdin.Close()
		return nil, e
	}
	if e = cmd.Start(); e != nil {
		stdin.Close()
		stdout.Close()
		return nil, e
	}
	p := &Process{Manifest: m, cmd: cmd, stdin: stdin, stdout: stdout, done: make(chan struct{}), stopped: make(chan struct{}), StartedAt: time.Now(), stderr: tail}
	go func() { p.waitErr = cmd.Wait(); p.ready.Store(false); close(p.done) }()
	cleanup := true
	defer func() {
		if cleanup {
			p.Stop(context.Background())
		}
	}()
	type bootstrap struct {
		APIVersion int    `json:"api_version"`
		Address    string `json:"address"`
	}
	type result struct {
		boot bootstrap
		err  error
	}
	bootCh := make(chan result, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 4096), 4096)
		var r result
		if !sc.Scan() {
			r.err = fmt.Errorf("plugin exited before bootstrap")
		} else {
			r.err = json.Unmarshal(sc.Bytes(), &r.boot)
		}
		bootCh <- r
	}()
	var boot bootstrap
	select {
	case r := <-bootCh:
		if r.err != nil {
			return nil, r.err
		}
		boot = r.boot
	case <-startCtx.Done():
		return nil, startCtx.Err()
	case <-p.done:
		return nil, fmt.Errorf("plugin exited during startup")
	}
	host, port, e := net.SplitHostPort(boot.Address)
	n, pe := strconv.Atoi(port)
	if e != nil || pe != nil || n < 1 || n > 65535 || host != "127.0.0.1" || boot.APIVersion != sdk.APIVersion {
		return nil, fmt.Errorf("invalid plugin bootstrap endpoint")
	}
	authorize := func(ctx context.Context) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	conn, e := grpc.NewClient("passthrough:///"+boot.Address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithNoProxy(), grpc.WithDisableRetry(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(sdk.MaxMessageBytes), grpc.MaxCallSendMsgSize(sdk.MaxMessageBytes)),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoke(authorize(ctx), method, req, reply, cc, opts...)
		}),
		grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(authorize(ctx), desc, cc, method, opts...)
		}))
	if e != nil {
		return nil, e
	}
	p.Conn = conn
	api := pb.NewLifecycleClient(conn)
	info, e := api.Handshake(startCtx, &pb.Hello{ApiVersion: sdk.APIVersion, GatewayVersion: sdk.GatewayVersion, Capabilities: sdk.HostCapabilities})
	if e != nil {
		return nil, fmt.Errorf("handshake %s: RPC %s", m.ID, status.Code(e))
	}
	if info.Id != m.ID || info.Version != m.Version || info.ApiVersion != uint32(m.APIVersion) || !same(info.Provides, m.Provides) || !same(info.Capabilities, m.Capabilities) {
		return nil, fmt.Errorf("handshake identity/version/capabilities mismatch for %s", m.ID)
	}
	p.Info = info
	if _, e = api.Configure(startCtx, &pb.Configuration{Json: config}); e != nil {
		return nil, fmt.Errorf("configure %s: RPC %s", m.ID, status.Code(e))
	}
	h, e := api.Health(startCtx, &pb.Empty{})
	if e != nil || !h.GetReady() {
		return nil, fmt.Errorf("plugin %s not healthy", m.ID)
	}
	select {
	case <-p.done:
		return nil, fmt.Errorf("plugin exited at readiness")
	default:
	}
	p.ready.Store(true)
	healthCtx, stop := context.WithCancel(context.Background())
	p.stopMonitor = stop
	p.monitorDone = make(chan struct{})
	go p.monitor(healthCtx)
	cleanup = false
	return p, nil
}
func same(a, b []string) bool {
	aa, bb := slices.Clone(a), slices.Clone(b)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}
func (p *Process) monitor(ctx context.Context) {
	defer close(p.monitorDone)
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			return
		case <-t.C:
			c, cancel := context.WithTimeout(ctx, time.Second)
			h, e := pb.NewLifecycleClient(p.Conn).Health(c, &pb.Empty{})
			cancel()
			p.ready.Store(e == nil && h.GetReady())
		}
	}
}

// Stop is idempotent, drains via RPC, then kills after a bounded grace period.
func (p *Process) Stop(ctx context.Context) {
	p.stopOnce.Do(func() {
		defer close(p.stopped)
		p.stopping.Store(true)
		p.ready.Store(false)
		if p.monitorDone == nil {
			// Failed startup has no admitted requests to drain.
			_ = p.cmd.Process.Kill()
			<-p.done
			_ = p.stdin.Close()
			_ = p.stdout.Close()
			if p.Conn != nil {
				_ = p.Conn.Close()
			}
			return
		}
		if p.stopMonitor != nil {
			p.stopMonitor()
			<-p.monitorDone
		}
		select {
		case <-p.done:
			_ = p.stdin.Close()
			_ = p.stdout.Close()
			if p.Conn != nil {
				_ = p.Conn.Close()
			}
			return
		default:
		}
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if p.Conn != nil {
			_, _ = pb.NewLifecycleClient(p.Conn).Shutdown(c, &pb.Empty{})
		}
		select {
		case <-p.done:
		case <-c.Done():
			_ = p.cmd.Process.Kill()
			<-p.done
		}
		_ = p.stdin.Close()
		_ = p.stdout.Close()
		if p.Conn != nil {
			_ = p.Conn.Close()
		}
	})
	<-p.stopped
}

type Manager struct {
	mu        sync.RWMutex
	ops       sync.Mutex
	items     map[string]*Process
	manifests map[string]manifest.Manifest
	configs   map[string]json.RawMessage
	closed    bool
	restart   RestartOptions
	states    map[string]*restartState
	ctx       context.Context
	cancel    context.CancelFunc
	watchers  sync.WaitGroup
}

func NewManager() *Manager {
	return NewManagerWithOptions(DefaultRestartOptions())
}
func (m *Manager) Start(ctx context.Context, list []manifest.Manifest, configs map[string]json.RawMessage) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	m.mu.RLock()
	invalid := m.closed || len(m.items) != 0
	m.mu.RUnlock()
	if invalid {
		return fmt.Errorf("manager already started/closed")
	}
	for _, decl := range list {
		m.mu.RLock()
		_, duplicate := m.items[decl.ID]
		m.mu.RUnlock()
		if duplicate {
			m.stopAll(ctx)
			return fmt.Errorf("duplicate plugin %s", decl.ID)
		}
		p, e := Start(ctx, decl, configs[decl.ID])
		if e != nil {
			m.stopAll(context.Background())
			return e
		}
		m.mu.Lock()
		m.items[decl.ID] = p
		m.manifests[decl.ID] = decl
		m.configs[decl.ID] = configs[decl.ID]
		m.mu.Unlock()
	}
	for _, p := range m.List() {
		m.watch(p)
	}
	return nil
}
func (m *Manager) Get(id string) (*Process, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.items[id]
	if m.closed || !ok || !p.Ready() {
		return nil, fmt.Errorf("plugin %s unavailable", id)
	}
	return p, nil
}
func (m *Manager) List() []*Process {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var r []*Process
	for _, p := range m.items {
		r = append(r, p)
	}
	slices.SortFunc(r, func(a, b *Process) int { return strings.Compare(a.Manifest.ID, b.Manifest.ID) })
	return r
}
func (m *Manager) Restart(ctx context.Context, id string) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	m.mu.RLock()
	p, ok := m.items[id]
	closed := m.closed
	m.mu.RUnlock()
	if closed || !ok {
		return fmt.Errorf("unknown/closed plugin %s", id)
	}
	if err := m.replace(ctx, id, p); err != nil {
		return err
	}
	m.mu.Lock()
	m.states[id] = &restartState{}
	next := m.items[id]
	m.mu.Unlock()
	m.watch(next)
	return nil
}

// replace runs under ops and publishes only a fully negotiated/configured process.
func (m *Manager) replace(ctx context.Context, id string, p *Process) error {
	m.mu.RLock()
	decl, config := m.manifests[id], m.configs[id]
	m.mu.RUnlock()
	p.Stop(ctx)
	next, e := Start(ctx, decl, config)
	if e != nil {
		return e
	}
	// The registry is immutable for this boot. A restart may not change bindings.
	oldInfo, _ := json.Marshal(p.Info)
	newInfo, _ := json.Marshal(next.Info)
	if string(oldInfo) != string(newInfo) {
		next.Stop(context.Background())
		return fmt.Errorf("plugin description changed during restart")
	}
	m.mu.Lock()
	m.items[id] = next
	m.mu.Unlock()
	return nil
}
func (m *Manager) stopAll(ctx context.Context) {
	for _, p := range m.List() {
		p.Stop(ctx)
	}
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
}
func (m *Manager) Close(ctx context.Context) {
	m.cancel()
	m.ops.Lock()
	m.stopAll(ctx)
	m.ops.Unlock()
	m.watchers.Wait()
}
