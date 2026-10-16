// Package registry binds negotiated capabilities, independent of plugin identities.
package registry

import (
	"fmt"
	"strings"

	"github.com/huey1in/moduapi/runtime/process"
	"github.com/huey1in/moduapi/runtime/rpc"
	"github.com/huey1in/moduapi/sdk"
	"google.golang.org/grpc"
)

type binding struct{ plugin, protocol string }
type Registry struct {
	manager   *process.Manager
	roles     map[string]map[string]bool
	endpoints map[string]binding
	protocols map[string]string
}

func New(m *process.Manager) (*Registry, error) {
	r := &Registry{manager: m, roles: map[string]map[string]bool{}, endpoints: map[string]binding{}, protocols: map[string]string{}}
	for _, p := range m.List() {
		roles := map[string]bool{}
		for _, role := range p.Info.Provides {
			switch role {
			case sdk.KindProtocol, sdk.KindUpstream, sdk.KindScheduler:
			default:
				return nil, fmt.Errorf("plugin kind %s has no v1 service", role)
			}
			roles[role] = true
		}
		r.roles[p.Info.Id] = roles
		if !roles[sdk.KindProtocol] && len(p.Info.Endpoints) > 0 {
			return nil, fmt.Errorf("non-protocol plugin declares endpoints")
		}
		if roles[sdk.KindProtocol] && len(p.Info.Endpoints) == 0 {
			return nil, fmt.Errorf("protocol plugin must declare endpoints")
		}
		for _, ep := range p.Info.Endpoints {
			if ep == nil || ep.Method == "" || ep.Method != strings.ToUpper(ep.Method) || strings.ContainsAny(ep.Method, " \r\n\t") || !strings.HasPrefix(ep.Path, "/") || strings.ContainsAny(ep.Path, "?#\r\n") || ep.Protocol == "" || ep.Path == "/healthz" || ep.Path == "/readyz" {
				return nil, fmt.Errorf("invalid protocol endpoint")
			}
			key := ep.Method + " " + ep.Path
			if _, ok := r.endpoints[key]; ok {
				return nil, fmt.Errorf("duplicate endpoint %s", key)
			}
			if old, ok := r.protocols[ep.Protocol]; ok && old != p.Info.Id {
				return nil, fmt.Errorf("duplicate protocol %s", ep.Protocol)
			}
			r.protocols[ep.Protocol] = p.Info.Id
			r.endpoints[key] = binding{p.Info.Id, ep.Protocol}
		}
	}
	return r, nil
}
func (r *Registry) Has(id, role string) bool   { return r.roles[id][role] }
func (r *Registry) HasProtocol(id string) bool { _, ok := r.protocols[id]; return ok }
func (r *Registry) Endpoint(method, path string) (string, string, bool) {
	b, ok := r.endpoints[method+" "+path]
	return b.plugin, b.protocol, ok
}
func (r *Registry) resolve(id, role string) (rpc.Resolve, error) {
	if !r.Has(id, role) {
		return nil, fmt.Errorf("plugin %s does not provide %s", id, role)
	}
	return func() (*grpc.ClientConn, error) {
		p, e := r.manager.Get(id)
		if e != nil {
			return nil, e
		}
		return p.Conn, nil
	}, nil
}
func (r *Registry) Protocol(id string) (sdk.ProtocolClient, error) {
	f, e := r.resolve(id, sdk.KindProtocol)
	return rpc.Protocol{Resolve: f}, e
}
func (r *Registry) Upstream(id string) (sdk.UpstreamClient, error) {
	f, e := r.resolve(id, sdk.KindUpstream)
	return rpc.Upstream{Resolve: f}, e
}
func (r *Registry) Scheduler(id string) (sdk.Scheduler, error) {
	f, e := r.resolve(id, sdk.KindScheduler)
	return rpc.Scheduler{Resolve: f}, e
}
func (r *Registry) Ready() bool {
	for _, p := range r.manager.List() {
		if !p.Ready() {
			return false
		}
	}
	return true
}
