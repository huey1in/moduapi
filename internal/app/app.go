// Package app is the composition root. Only this layer knows core and runtime.
package app

import (
	"context"
	"fmt"
	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/core/gateway"
	"github.com/huey1in/moduapi/core/lifecycle"
	"github.com/huey1in/moduapi/runtime/manifest"
	"github.com/huey1in/moduapi/runtime/process"
	"github.com/huey1in/moduapi/runtime/registry"
)

type App struct {
	Gateway  *gateway.Gateway
	Manager  *process.Manager
	Registry *registry.Registry
}

func Start(ctx context.Context, c config.Config, hooks *lifecycle.Bus) (*App, error) {
	c.Defaults()
	if e := c.Validate(); e != nil {
		return nil, e
	}
	list, e := manifest.Discover(c.Plugins.Directory)
	if e != nil {
		return nil, e
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no plugins discovered; run the build command first")
	}
	m := process.NewManager()
	if e = m.Start(ctx, list, c.PluginConfigs()); e != nil {
		return nil, e
	}
	fail := true
	defer func() {
		if fail {
			m.Close(context.Background())
		}
	}()
	r, e := registry.New(m)
	if e != nil {
		return nil, e
	}
	if e = c.ValidateReferences(r.Has, r.HasProtocol); e != nil {
		return nil, e
	}
	g, e := gateway.New(c, r, hooks, nil)
	if e != nil {
		return nil, e
	}
	fail = false
	return &App{Gateway: g, Manager: m, Registry: r}, nil
}
