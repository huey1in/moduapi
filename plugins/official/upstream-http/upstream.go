// Package httpupstream is a generic HTTP transport. It has no model vendor logic.
package httpupstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/huey1in/moduapi/internal/httpwire"
	"github.com/huey1in/moduapi/sdk"
)

type Upstream struct {
	client    *http.Client
	transport *http.Transport
	network   []string
}
type config struct {
	BaseURL   string            `json:"base_url"`
	APIKeyEnv string            `json:"api_key_env"`
	Headers   map[string]string `json:"headers"`
}

func New(network []string) *Upstream {
	t := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ForceAttemptHTTP2: true, MaxIdleConns: 100, MaxIdleConnsPerHost: 32, MaxConnsPerHost: 64, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 30 * time.Second, DisableCompression: true}
	return &Upstream{transport: t, client: &http.Client{Transport: t, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, network: append([]string(nil), network...)}
}
func (u *Upstream) Close() error { u.transport.CloseIdleConnections(); return nil }
func (u *Upstream) allowed(host string) bool {
	host = strings.ToLower(host)
	for _, rule := range u.network {
		rule = strings.ToLower(rule)
		if rule == "*" || rule == host {
			return true
		}
		if strings.HasPrefix(rule, "*.") && strings.HasSuffix(host, rule[1:]) && host != rule[2:] {
			return true
		}
	}
	return false
}
func (u *Upstream) Execute(ctx context.Context, in *sdk.Execution, send sdk.Send) error {
	var c config
	d := json.NewDecoder(bytes.NewReader(in.ConfigJson))
	d.DisallowUnknownFields()
	bad := func() error {
		return sdk.Fail(sdk.Local, "invalid_upstream_config", "invalid HTTP upstream configuration", 500)
	}
	if d.Decode(&c) != nil {
		return bad()
	}
	base, e := url.Parse(c.BaseURL)
	if e != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return bad()
	}
	if !u.allowed(base.Hostname()) {
		return sdk.Fail(sdk.Local, "network_denied", "upstream host not granted by manifest", 500)
	}
	r := in.Request
	if r == nil || !strings.HasPrefix(r.Path, "/") || strings.ContainsAny(r.Path, "?#\\\r\n") || strings.Contains(r.Path, "..") {
		return bad()
	}
	req, e := http.NewRequestWithContext(ctx, r.Method, strings.TrimRight(base.String(), "/")+r.Path, bytes.NewReader(r.Body))
	if e != nil {
		return bad()
	}
	req.GetBody = nil
	req.Header = httpwire.Clean(sdk.HTTPHeaders(r.Headers))
	req.Header.Del("Authorization")
	req.Header.Del("Cookie")
	req.Header.Del("Host")
	req.Header.Set("Accept-Encoding", "identity")
	for k, v := range c.Headers {
		if strings.ContainsAny(k+v, "\r\n") || k == "" {
			return bad()
		}
		req.Header.Set(k, v)
	}
	req.Header = httpwire.Clean(req.Header)
	req.Header.Del("Host")
	if c.APIKeyEnv != "" {
		key, ok := os.LookupEnv(c.APIKeyEnv)
		if !ok || key == "" || strings.ContainsAny(key, "\r\n") {
			return sdk.Fail(sdk.Local, "secret_unavailable", "upstream credential unavailable", 500)
		}
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, e := u.client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		kind := sdk.Transport
		var ne net.Error
		if errors.As(e, &ne) && ne.Timeout() {
			kind = sdk.Timeout
		}
		return &sdk.Error{Failure: &sdk.Failure{Kind: kind, Code: "upstream_connection", Message: "upstream connection failed", Status: 502, RequestSent: true}}
	}
	defer resp.Body.Close()
	if e = send(sdk.Start(resp.StatusCode, httpwire.Clean(resp.Header))); e != nil {
		return e
	}
	buf := make([]byte, sdk.MaxChunkBytes)
	for {
		n, e := resp.Body.Read(buf)
		if n > 0 {
			if se := send(sdk.Data(buf[:n])); se != nil {
				return se
			}
		}
		if e == io.EOF {
			return send(sdk.End(nil))
		}
		if e != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return &sdk.Error{Failure: &sdk.Failure{Kind: sdk.Transport, Code: "upstream_read", Message: "upstream disconnected", Status: 502, RequestSent: true}}
		}
	}
}
