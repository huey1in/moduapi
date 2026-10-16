// Package mockupstream is a deterministic, local OpenAI-compatible test endpoint.
package mockupstream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req) != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if req.Model == "mock-4xx" || req.Model == "mock-5xx" {
		status := 429
		if req.Model == "mock-5xx" {
			status = 503
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, `{"error":{"message":"mock upstream rejection"}}`)
		return
	}
	if req.Model == "mock-timeout" {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
	if !req.Stream {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "mock-chat", "object": "chat.completion", "model": req.Model, "choices": []any{map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": "Hello from ModuAPI mock"}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 3, "completion_tokens": 5}})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	f, ok := w.(http.Flusher)
	if !ok {
		return
	}
	for _, chunk := range []string{`{"id":"mock-chat","choices":[{"index":0,"delta":{"content":"Hello"}}]}`, `{"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":5}}`, `[DONE]`} {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(20 * time.Millisecond):
		}
		if _, e := fmt.Fprintf(w, "data: %s\n\n", chunk); e != nil {
			return
		}
		f.Flush()
	}
}
