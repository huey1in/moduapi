// Package config describes kernel wiring. Plugin configuration is opaque JSON.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/huey1in/moduapi/core/router"
	"go.yaml.in/yaml/v3"
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, e := time.ParseDuration(n.Value)
	if e != nil {
		return e
	}
	*d = Duration(v)
	return nil
}

type Object map[string]any

func (o Object) JSON() json.RawMessage {
	if o == nil {
		return json.RawMessage("{}")
	}
	b, _ := json.Marshal(o)
	return b
}

type Config struct {
	Server    Server              `yaml:"server"`
	Plugins   Plugins             `yaml:"plugins"`
	Routes    []router.Route      `yaml:"routes"`
	Upstreams map[string]Upstream `yaml:"upstreams"`
}
type Server struct {
	Listen           string   `yaml:"listen"`
	MaxBodyBytes     int64    `yaml:"max_body_bytes"`
	RequestTimeout   Duration `yaml:"request_timeout"`
	FirstByteTimeout Duration `yaml:"first_byte_timeout"`
	IdleTimeout      Duration `yaml:"idle_timeout"`
	WriteTimeout     Duration `yaml:"write_timeout"`
}
type Plugins struct {
	Directory string            `yaml:"directory"`
	Config    map[string]Object `yaml:"config"`
}
type Upstream struct {
	Plugin         string            `yaml:"plugin"`
	Config         Object            `yaml:"config"`
	ProtocolConfig Object            `yaml:"protocol_config"`
	Metadata       map[string]string `yaml:"metadata"`
}

func (c *Config) Defaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = "127.0.0.1:8080"
	}
	if c.Plugins.Directory == "" {
		c.Plugins.Directory = "./dist/plugins"
	}
	if c.Server.MaxBodyBytes == 0 {
		c.Server.MaxBodyBytes = 4 << 20
	}
	for p, def := range map[*Duration]time.Duration{&c.Server.RequestTimeout: 5 * time.Minute, &c.Server.FirstByteTimeout: 30 * time.Second, &c.Server.IdleTimeout: 30 * time.Second, &c.Server.WriteTimeout: 15 * time.Second} {
		if *p == 0 {
			*p = Duration(def)
		}
	}
	for i := range c.Routes {
		if c.Routes[i].Retry.MaxAttempts == 0 {
			c.Routes[i].Retry.MaxAttempts = 1
		}
	}
}
func (c *Config) Validate() error {
	if c.Server.MaxBodyBytes < 1 || c.Server.MaxBodyBytes > 8<<20 {
		return fmt.Errorf("max_body_bytes must be 1..8MiB")
	}
	for _, d := range []Duration{c.Server.RequestTimeout, c.Server.FirstByteTimeout, c.Server.IdleTimeout, c.Server.WriteTimeout} {
		if d <= 0 {
			return fmt.Errorf("timeouts must be positive")
		}
	}
	if len(c.Routes) == 0 {
		return fmt.Errorf("at least one route is required")
	}
	for id, u := range c.Upstreams {
		if id == "" || u.Plugin == "" {
			return fmt.Errorf("invalid upstream")
		}
		for _, obj := range []Object{u.Config, u.ProtocolConfig} {
			if _, e := json.Marshal(obj); e != nil {
				return fmt.Errorf("upstream %s: %w", id, e)
			}
		}
	}
	for id, obj := range c.Plugins.Config {
		if id == "" {
			return fmt.Errorf("empty plugin config id")
		}
		if _, e := json.Marshal(obj); e != nil {
			return e
		}
	}
	_, e := router.New(c.Routes)
	if e != nil {
		return e
	}
	for _, r := range c.Routes {
		for _, id := range r.Upstreams {
			if _, ok := c.Upstreams[id]; !ok {
				return fmt.Errorf("route %s references unknown upstream %s", r.ID, id)
			}
		}
	}
	return nil
}
func Load(path string) (Config, error) {
	var c Config
	f, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if e != nil {
		return c, e
	}
	if len(data) > 1<<20 {
		return c, fmt.Errorf("config exceeds 1MiB")
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	var tail any
	if e = d.Decode(&tail); e != io.EOF {
		return c, fmt.Errorf("config must have one document")
	}
	c.Defaults()
	if e = c.Validate(); e != nil {
		return c, e
	}
	if !filepath.IsAbs(c.Plugins.Directory) {
		abs, e := filepath.Abs(filepath.Join(filepath.Dir(path), c.Plugins.Directory))
		if e != nil {
			return c, e
		}
		c.Plugins.Directory = abs
	}
	return c, nil
}
func (c Config) PluginConfigs() map[string]json.RawMessage {
	r := map[string]json.RawMessage{}
	for k, v := range c.Plugins.Config {
		r[k] = v.JSON()
	}
	return r
}
func (c Config) ValidateReferences(has func(string, string) bool, protocol func(string) bool) error {
	for _, r := range c.Routes {
		if !protocol(r.Protocol) || !has(r.Scheduler, "scheduler") {
			return fmt.Errorf("route %s references an unregistered protocol/scheduler", r.ID)
		}
	}
	for id, u := range c.Upstreams {
		if !has(u.Plugin, "upstream") {
			return fmt.Errorf("upstream %s references an unregistered plugin", id)
		}
	}
	for id := range c.Plugins.Config {
		found := false
		for _, role := range []string{"protocol", "upstream", "scheduler"} {
			found = found || has(id, role)
		}
		if !found {
			return fmt.Errorf("configuration for unknown plugin %s", strings.TrimSpace(id))
		}
	}
	return nil
}
