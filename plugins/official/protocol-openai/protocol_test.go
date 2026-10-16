package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/huey1in/moduapi/sdk"
)

func TestOpaqueDecodeAndMapping(t *testing.T) {
	p := Protocol{}
	b := []byte(`{"model":"alias", "messages":[{"role":"user","content":"hi"}],"vendor_extension":{"x":1}}`)
	r, e := p.Decode(context.Background(), &sdk.Ingress{Id: "id", Headers: sdk.Headers(http.Header{"Content-Type": []string{"application/json"}}), Body: b})
	if e != nil {
		t.Fatal(e)
	}
	out, e := p.Prepare(context.Background(), &sdk.Preparation{Request: r, OptionsJson: []byte(`{}`)})
	if e != nil || string(out.Body) != string(b) {
		t.Fatal("opaque body was re-encoded")
	}
	out, e = p.Prepare(context.Background(), &sdk.Preparation{Request: r, OptionsJson: []byte(`{"model_mapping":{"alias":"actual"}}`)})
	if e != nil || !strings.Contains(string(out.Body), `"model":"actual"`) || out.Model != "alias" || string(r.Body) != string(b) {
		t.Fatalf("invalid mapping: %v %v", out, e)
	}
}
func TestFragmentedSSEUsageAndTerminal(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"CRLF fragments", "data: {\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":4,\"prompt_tokens_details\":{\"cached_tokens\":3}}}\r\n\r\ndata: [DONE]\r\n\r\n", true},
		{"missing DONE", "data: {\"choices\":[]}\n\n", false},
		{"malformed JSON", "data: {bad}\n\n", false},
		{"error event", "data: {\"error\":{\"message\":\"failure\"}}\n\n", false},
		{"unterminated event", "data: [DONE]\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := []*sdk.Frame{sdk.Start(200, http.Header{"Content-Type": []string{"text/event-stream"}}), sdk.Data(nil)}
			for _, b := range []byte(tc.data) {
				frames = append(frames, sdk.Data([]byte{b}))
			}
			frames = append(frames, sdk.End(nil))
			i := 0
			var usage *sdk.Usage
			e := (Protocol{}).Transform(context.Background(), &sdk.Request{Stream: true}, func() (*sdk.Frame, error) {
				if i >= len(frames) {
					return nil, io.EOF
				}
				f := frames[i]
				i++
				return f, nil
			}, func(f *sdk.Frame) error {
				if f.GetEnd() != nil {
					usage = f.GetEnd().Usage
				}
				return nil
			})
			if (e == nil) != tc.valid {
				t.Fatalf("%v", e)
			}
			if tc.valid && (usage == nil || usage.InputTokens != 11 || usage.OutputTokens != 4 || usage.CachedTokens != 3) {
				t.Fatalf("usage %v", usage)
			}
		})
	}
}
