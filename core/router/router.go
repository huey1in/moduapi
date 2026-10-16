// Package router selects a candidate set, never a candidate or scoring formula.
package router

import (
	"context"
	"fmt"
	"strings"

	"github.com/huey1in/moduapi/sdk"
)

type Retry struct {
	MaxAttempts int  `yaml:"max_attempts"`
	AllowReplay bool `yaml:"allow_replay"`
}
type Route struct {
	ID        string   `yaml:"id"`
	Protocol  string   `yaml:"protocol"`
	Model     string   `yaml:"model"`
	Upstreams []string `yaml:"upstreams"`
	Scheduler string   `yaml:"scheduler"`
	Retry     Retry    `yaml:"retry"`
}
type Router struct{ routes []Route }

func New(routes []Route) (*Router, error) {
	ids := map[string]bool{}
	var result []Route
	for _, r := range routes {
		if r.ID == "" || ids[r.ID] || r.Protocol == "" || r.Scheduler == "" || r.Model == "" || len(r.Upstreams) == 0 {
			return nil, fmt.Errorf("invalid/duplicate route %q", r.ID)
		}
		ids[r.ID] = true
		if strings.Contains(r.Model, "*") && r.Model != "*" {
			return nil, fmt.Errorf("v1 models support exact match or *")
		}
		if r.Retry.MaxAttempts < 1 || r.Retry.MaxAttempts > 8 {
			return nil, fmt.Errorf("retry max_attempts must be 1..8")
		}
		seen := map[string]bool{}
		for _, id := range r.Upstreams {
			if id == "" || seen[id] {
				return nil, fmt.Errorf("invalid/duplicate upstream in route")
			}
			seen[id] = true
		}
		r.Upstreams = append([]string(nil), r.Upstreams...)
		result = append(result, r)
	}
	return &Router{routes: result}, nil
}

// First matching route wins, making configuration order explicit.
func (r *Router) Match(ctx context.Context, req *sdk.Request) (Route, error) {
	if e := ctx.Err(); e != nil {
		return Route{}, e
	}
	for _, route := range r.routes {
		if route.Protocol == req.Protocol && (route.Model == "*" || route.Model == req.Model) {
			return route, nil
		}
	}
	return Route{}, sdk.Fail(sdk.Local, "route_not_found", "no matching route", 404)
}
