// Package httpwire contains transport hygiene, shared by host and official plugins.
package httpwire

import (
	"net/http"
	"strings"
)

// Clean removes connection-specific headers, including Connection-nominated fields.
func Clean(h http.Header) http.Header {
	r := make(http.Header, len(h))
	for k, values := range h {
		for _, value := range values {
			r.Add(k, value)
		}
	}
	for _, line := range r.Values("Connection") {
		for _, k := range strings.Split(line, ",") {
			r.Del(strings.TrimSpace(k))
		}
	}
	for _, k := range []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade", "Content-Length"} {
		r.Del(k)
	}
	return r
}
