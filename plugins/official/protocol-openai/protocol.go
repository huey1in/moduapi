// Package openai implements the OpenAI Chat protocol outside the kernel.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/huey1in/moduapi/sdk"
	"google.golang.org/protobuf/proto"
)

type Protocol struct{}

func (Protocol) Decode(ctx context.Context, in *sdk.Ingress) (*sdk.Request, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	bad := func() (*sdk.Request, error) {
		return nil, sdk.Fail(sdk.Local, "invalid_request", "expected a JSON chat request with model and non-empty messages", 400)
	}
	ct, _, e := mime.ParseMediaType(sdk.HTTPHeaders(in.Headers).Get("Content-Type"))
	if e != nil || ct != "application/json" {
		return nil, sdk.Fail(sdk.Local, "unsupported_media_type", "Content-Type must be application/json", 415)
	}
	d := json.NewDecoder(bytes.NewReader(in.Body))
	tok, e := d.Token()
	if e != nil || tok != json.Delim('{') {
		return bad()
	}
	seen := map[string]bool{}
	var model string
	var stream bool
	messages := false
	for d.More() {
		t, e := d.Token()
		if e != nil {
			return bad()
		}
		key, ok := t.(string)
		if !ok || seen[key] {
			return bad()
		}
		seen[key] = true
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return bad()
		}
		switch key {
		case "model":
			if json.Unmarshal(raw, &model) != nil {
				return bad()
			}
		case "stream":
			if string(raw) == "null" || json.Unmarshal(raw, &stream) != nil {
				return bad()
			}
		case "messages":
			var a []json.RawMessage
			if json.Unmarshal(raw, &a) != nil || len(a) == 0 {
				return bad()
			}
			messages = true
		}
	}
	if _, e = d.Token(); e != nil {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF || !messages || strings.TrimSpace(model) == "" || len(model) > 256 || strings.ContainsAny(model, "\r\n\x00") {
		return bad()
	}
	h := http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{"application/json"}}
	if stream {
		h.Set("Accept", "text/event-stream")
	}
	var metadata map[string]string
	if key := sdk.HTTPHeaders(in.Headers).Get("X-Moduapi-Affinity"); len(key) > 0 && len(key) <= 512 {
		metadata = map[string]string{sdk.AffinityKey: key}
	}
	return &sdk.Request{Id: in.Id, Protocol: "openai.chat", Model: model, Stream: stream, Method: http.MethodPost, Path: "/chat/completions", Headers: sdk.Headers(h), Body: in.Body, Metadata: metadata}, nil
}
func (Protocol) Prepare(ctx context.Context, in *sdk.Preparation) (*sdk.Request, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	var opts struct {
		ModelMapping map[string]string `json:"model_mapping"`
	}
	d := json.NewDecoder(bytes.NewReader(in.OptionsJson))
	d.DisallowUnknownFields()
	if len(in.OptionsJson) > 0 {
		if d.Decode(&opts) != nil {
			return nil, sdk.Fail(sdk.Local, "invalid_protocol_config", "invalid protocol options", 500)
		}
	}
	mapped, ok := opts.ModelMapping[in.Request.Model]
	if !ok {
		return in.Request, nil
	}
	if mapped == "" {
		return nil, sdk.Fail(sdk.Local, "invalid_model_mapping", "mapped model is empty", 500)
	}
	r := proto.Clone(in.Request).(*sdk.Request)
	var fields map[string]json.RawMessage
	if json.Unmarshal(r.Body, &fields) != nil {
		return nil, sdk.Fail(sdk.Local, "invalid_payload", "invalid canonical payload", 500)
	}
	fields["model"], _ = json.Marshal(mapped)
	r.Body, _ = json.Marshal(fields)
	if r.Metadata == nil {
		r.Metadata = map[string]string{}
	}
	r.Metadata["upstream.model"] = mapped
	return r, nil
}
func (Protocol) Transform(ctx context.Context, req *sdk.Request, recv sdk.Receive, send sdk.Send) error {
	started := false
	success := false
	stream := false
	var body []byte
	o := observer{}
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		f, e := recv()
		if e == io.EOF {
			return sdk.Fail(sdk.ProtocolError, "missing_end", "upstream response ended without terminal frame", 502)
		}
		if e != nil {
			return e
		}
		switch {
		case f.GetStart() != nil:
			if started {
				return sdk.Fail(sdk.ProtocolError, "duplicate_start", "invalid upstream response sequence", 502)
			}
			started = true
			s := f.GetStart()
			success = s.Status >= 200 && s.Status < 300
			stream = req.Stream && success
			if stream {
				h := sdk.HTTPHeaders(s.Headers)
				ct, _, _ := mime.ParseMediaType(h.Get("Content-Type"))
				if ct != "text/event-stream" {
					return sdk.Fail(sdk.ProtocolError, "invalid_content_type", "expected upstream SSE response", 502)
				}
				h.Set("Cache-Control", "no-cache")
				h.Set("X-Accel-Buffering", "no")
				s.Headers = sdk.Headers(h)
			}
		case sdk.IsData(f):
			if !started || len(f.GetData()) > sdk.MaxChunkBytes {
				return sdk.Fail(sdk.ProtocolError, "invalid_chunk", "invalid upstream response frame", 502)
			}
			if stream {
				if e := o.feed(f.GetData()); e != nil {
					return o.failure(e)
				}
				if o.done {
					if len(bytes.TrimSpace(o.pending)) > 0 || len(o.event) > 0 {
						return o.failure(sdk.Fail(sdk.ProtocolError, "data_after_terminal", "upstream sent data after terminal event", 502))
					}
					if e := send(f); e != nil {
						return e
					}
					return send(sdk.End(o.usage))
				}
			} else if success {
				if len(body)+len(f.GetData()) > 4<<20 {
					return sdk.Fail(sdk.ProtocolError, "response_too_large", "JSON response exceeds 4MiB", 502)
				}
				body = append(body, f.GetData()...)
			}
		case f.GetEnd() != nil:
			if !started {
				return sdk.Fail(sdk.ProtocolError, "missing_start", "invalid upstream response sequence", 502)
			}
			if success {
				if stream {
					if !o.done || len(bytes.TrimSpace(o.pending)) > 0 || len(o.event) > 0 {
						return o.failure(sdk.Fail(sdk.ProtocolError, "incomplete_stream", "upstream SSE ended before [DONE]", 502))
					}
				} else {
					if !json.Valid(body) {
						return sdk.Fail(sdk.ProtocolError, "invalid_json_response", "upstream returned invalid JSON", 502)
					}
					o.extract(body)
				}
				return send(sdk.End(o.usage))
			}
			return send(f)
		case f.GetFailure() != nil:
			f.GetFailure().Usage = o.partial()
			return send(f)
		default:
			return sdk.Fail(sdk.ProtocolError, "invalid_frame", "invalid upstream response frame", 502)
		}
		if e := send(f); e != nil {
			return e
		}
	}
}

type observer struct {
	pending, event []byte
	done           bool
	usage          *sdk.Usage
}

func (o *observer) feed(b []byte) error {
	o.pending = append(o.pending, b...)
	for {
		idx := bytes.IndexByte(o.pending, '\n')
		if idx < 0 {
			break
		}
		line := bytes.TrimSuffix(o.pending[:idx], []byte{'\r'})
		if len(line) == 0 {
			if len(o.event) > 0 {
				data := bytes.TrimSpace(o.event)
				if bytes.Equal(data, []byte("[DONE]")) {
					o.done = true
				} else {
					if o.done || !json.Valid(data) {
						return sdk.Fail(sdk.ProtocolError, "invalid_sse", "invalid upstream SSE event", 502)
					}
					var check struct {
						Error json.RawMessage `json:"error"`
					}
					_ = json.Unmarshal(data, &check)
					if len(check.Error) > 0 && string(check.Error) != "null" {
						return sdk.Fail(sdk.ProtocolError, "upstream_error_event", "upstream reported a stream error", 502)
					}
					o.extract(data)
				}
				o.event = nil
			}
		} else if bytes.HasPrefix(line, []byte("data:")) {
			if len(o.event) > 0 {
				o.event = append(o.event, '\n')
			}
			o.event = append(o.event, bytes.TrimPrefix(line[5:], []byte{' '})...)
		}
		o.pending = o.pending[idx+1:]
		if len(o.event) > 1<<20 {
			return sdk.Fail(sdk.ProtocolError, "event_too_large", "upstream SSE event exceeds 1MiB", 502)
		}
	}
	if len(o.pending) > 1<<20 {
		return sdk.Fail(sdk.ProtocolError, "line_too_large", "upstream SSE line exceeds 1MiB", 502)
	}
	return nil
}
func (o *observer) extract(b []byte) {
	var envelope struct {
		Usage *struct {
			Input   int64 `json:"prompt_tokens"`
			Output  int64 `json:"completion_tokens"`
			Details struct {
				Cached int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(b, &envelope) == nil && envelope.Usage != nil {
		u := envelope.Usage
		o.usage = &sdk.Usage{InputTokens: max(0, u.Input), OutputTokens: max(0, u.Output), CachedTokens: max(0, u.Details.Cached)}
	}
}
func (o *observer) partial() *sdk.Usage {
	if o.usage == nil {
		return &sdk.Usage{Partial: true}
	}
	u := proto.Clone(o.usage).(*sdk.Usage)
	u.Partial = true
	return u
}
func (o *observer) failure(e error) error {
	f := sdk.FailureOf(e)
	f.Usage = o.partial()
	return &sdk.Error{Failure: f}
}
