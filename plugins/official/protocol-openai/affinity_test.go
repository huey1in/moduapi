package openai

import (
	"context"
	"github.com/huey1in/moduapi/sdk"
	"net/http"
	"strings"
	"testing"
)

func TestOpaqueAffinityHint(t *testing.T) {
	for _, key := range []string{"synthetic-session", strings.Repeat("x", 513)} {
		in := &sdk.Ingress{Id: "r", Headers: sdk.Headers(http.Header{"Content-Type": {"application/json"}, "X-Moduapi-Affinity": {key}}), Body: []byte(`{"model":"x","messages":[{}]}`)}
		r, e := (Protocol{}).Decode(context.Background(), in)
		if e != nil {
			t.Fatal(e)
		}
		if len(key) <= 512 && r.Metadata[sdk.AffinityKey] != key {
			t.Fatal("affinity lost")
		}
		if len(key) > 512 && r.Metadata[sdk.AffinityKey] != "" {
			t.Fatal("unbounded affinity")
		}
		if sdk.HTTPHeaders(r.Headers).Get("X-Moduapi-Affinity") != "" {
			t.Fatal("hint forwarded upstream")
		}
	}
}
