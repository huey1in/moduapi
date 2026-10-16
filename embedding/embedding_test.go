package embedding_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/core/router"
	"github.com/huey1in/moduapi/embedding"
	"github.com/huey1in/moduapi/internal/buildutil"
	"github.com/huey1in/moduapi/internal/mockupstream"
)

// This external package is intentionally written as a third-party host would
// write it: it owns the listener and composes the kernel as a Handler.
func TestExternalHostComposition(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	bundles := filepath.Join(t.TempDir(), "plugins")
	if err = buildutil.Build(root, bundles, false); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(mockupstream.Handler))
	defer upstream.Close()
	c := config.Config{
		Plugins:   config.Plugins{Directory: bundles},
		Routes:    []router.Route{{ID: "external", Protocol: "openai.chat", Model: "*", Scheduler: "scheduler.single", Upstreams: []string{"default"}}},
		Upstreams: map[string]config.Upstream{"default": {Plugin: "upstream.http", Config: config.Object{"base_url": upstream.URL + "/v1"}}},
	}
	rt, err := embedding.Start(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close(context.Background())
	if !rt.Ready() {
		t.Fatal("embedded runtime is not ready")
	}

	req := httptest.NewRequest(http.MethodPost, "http://host/v1/chat/completions", strings.NewReader(`{"model":"mock","messages":[{"role":"user","content":"hello"}],"stream":false}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	rt.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "Hello from ModuAPI mock") {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
}
