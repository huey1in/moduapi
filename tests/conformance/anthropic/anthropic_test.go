//go:build conformance

package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/core/router"
	"github.com/huey1in/moduapi/internal/app"
	"github.com/huey1in/moduapi/internal/buildutil"
	"github.com/huey1in/moduapi/runtime/manifest"
)

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(to, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentMessagesProcessAndGateway(t *testing.T) {
	python := os.Getenv("MODUAPI_CONFORMANCE_PYTHON")
	if python == "" {
		t.Skip("make conformance supplies the pinned Python interpreter")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	built := filepath.Join(t.TempDir(), "built")
	if err = buildutil.Build(root, built, false); err != nil {
		t.Fatal(err)
	}
	plugins := filepath.Join(t.TempDir(), "plugins")
	for _, id := range []string{"scheduler.single", "upstream.http"} {
		copyFile(t, filepath.Join(built, id, "manifest.yaml"), filepath.Join(plugins, id, "manifest.yaml"))
		copyFile(t, filepath.Join(built, id, "plugin"+buildutil.Suffix()), filepath.Join(plugins, id, "plugin"+buildutil.Suffix()))
	}
	ref := filepath.Join(root, "plugins", "reference", "protocol-anthropic")
	if err = filepath.Walk(ref, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if info.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(ref, path)
		if e != nil {
			return e
		}
		copyFile(t, path, filepath.Join(plugins, "protocol.anthropic-reference", rel))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(plugins, "protocol.anthropic-reference", "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	m.Runtime.Interpreter = python
	// The test uses a private copy so the production manifest remains PATH-based.
	data, _ := os.ReadFile(m.Path)
	data = []byte(strings.Replace(string(data), "interpreter: python", "interpreter: "+python, 1))
	if err = os.WriteFile(m.Path, data, 0644); err != nil {
		t.Fatal(err)
	}

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("Anthropic-Version") == "" {
			http.Error(w, "bad upstream request", 400)
			return
		}
		var req map[string]any
		if json.NewDecoder(r.Body).Decode(&req) != nil || req["max_tokens"] == nil || req["future_field"] == nil {
			http.Error(w, "bad JSON", 400)
			return
		}
		if req["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":0}}}\n\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":2}}\n\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "message", "content": []any{map[string]string{"type": "text", "text": "hello"}}, "usage": map[string]int{"input_tokens": 3, "output_tokens": 2}})
	}))
	defer up.Close()
	c := config.Config{Plugins: config.Plugins{Directory: plugins}, Routes: []router.Route{{ID: "messages", Protocol: "anthropic.messages", Model: "*", Scheduler: "scheduler.single", Upstreams: []string{"anthropic"}}}, Upstreams: map[string]config.Upstream{"anthropic": {Plugin: "upstream.http", Config: config.Object{"base_url": up.URL + "/v1"}}}}
	c.Defaults()
	a, err := app.Start(context.Background(), c, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Manager.Close(context.Background())
	for _, stream := range []bool{false, true} {
		body, _ := json.Marshal(map[string]any{"model": "claude-reference", "max_tokens": 16, "messages": []any{map[string]any{"role": "user", "content": "hello"}}, "stream": stream, "future_field": map[string]any{"kept": true}})
		req := httptest.NewRequest(http.MethodPost, "http://gateway/v1/messages", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.Gateway.ServeHTTP(w, req)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "hello") {
			t.Fatalf("stream=%v status=%d body=%s", stream, w.Code, w.Body.String())
		}
	}
}
