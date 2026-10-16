package httpupstream

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/huey1in/moduapi/sdk"
)

func TestConnectionReuseAndHeaderIsolation(t *testing.T) {
	t.Setenv("MODUAPI_UNIT_TEST_KEY", "upstream-key")
	var connections atomic.Int64
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.Header.Get("Authorization") != "Bearer upstream-key" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Hop") != "" {
			t.Errorf("bad headers %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	s.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	s.Start()
	defer s.Close()
	u := New([]string{"127.0.0.1"})
	defer u.Close()
	c, _ := json.Marshal(config{BaseURL: s.URL, APIKeyEnv: "MODUAPI_UNIT_TEST_KEY"})
	for range 4 {
		var start, end bool
		e := u.Execute(context.Background(), &sdk.Execution{Request: &sdk.Request{Method: "POST", Path: "/chat", Body: []byte(`{}`), Headers: sdk.Headers(http.Header{"Authorization": []string{"Bearer downstream"}, "Cookie": []string{"private"}, "Connection": []string{"X-Hop"}, "X-Hop": []string{"bad"}})}, ConfigJson: c}, func(f *sdk.Frame) error {
			start = start || f.GetStart() != nil
			end = end || f.GetEnd() != nil
			return nil
		})
		if e != nil || !start || !end {
			t.Fatalf("%v start=%v end=%v", e, start, end)
		}
	}
	if connections.Load() != 1 {
		t.Fatalf("transport was not reused: %d connections", connections.Load())
	}
}
func TestRedirectAndPermissions(t *testing.T) {
	var hits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer redirect.Close()
	u := New([]string{"127.0.0.1"})
	defer u.Close()
	c, _ := json.Marshal(config{BaseURL: redirect.URL})
	var status int32
	e := u.Execute(context.Background(), &sdk.Execution{Request: &sdk.Request{Method: "POST", Path: "/chat", Body: []byte(`{}`)}, ConfigJson: c}, func(f *sdk.Frame) error {
		if f.GetStart() != nil {
			status = f.GetStart().Status
		}
		return nil
	})
	if e != nil || status != 302 || hits.Load() != 0 {
		t.Fatalf("redirect followed: %v %d %d", e, status, hits.Load())
	}
	denied := New(nil)
	defer denied.Close()
	e = denied.Execute(context.Background(), &sdk.Execution{Request: &sdk.Request{Method: "POST", Path: "/chat"}, ConfigJson: c}, func(*sdk.Frame) error { return nil })
	if e == nil || sdk.FailureOf(e).Code != "network_denied" {
		t.Fatalf("network contract not checked: %v", e)
	}
	wild := New([]string{"*.example.com"})
	defer wild.Close()
	if !wild.allowed("api.example.com") || wild.allowed("example.com") || wild.allowed("evil-example.com") {
		t.Fatal("wildcard matched wrong host")
	}
}
