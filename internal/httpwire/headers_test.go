package httpwire

import (
	"net/http"
	"testing"
)

func TestConnectionNominatedHeadersAreCaseInsensitive(t *testing.T) {
	h := http.Header{"connection": []string{"x-private, X-Other"}, "x-private": []string{"secret"}, "X-Other": []string{"private"}, "content-length": []string{"999"}, "set-cookie": []string{"a=1", "b=2"}}
	clean := Clean(h)
	if clean.Get("Connection") != "" || clean.Get("X-Private") != "" || clean.Get("X-Other") != "" || clean.Get("Content-Length") != "" || len(clean.Values("Set-Cookie")) != 2 {
		t.Fatalf("bad header sanitation: %v", clean)
	}
	if h["x-private"][0] != "secret" {
		t.Fatal("input was mutated")
	}
}
