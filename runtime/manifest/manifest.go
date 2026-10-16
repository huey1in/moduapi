// Package manifest validates declarations without executing plugin code.
package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/huey1in/moduapi/sdk"
	"go.yaml.in/yaml/v3"
)

type Manifest struct {
	APIVersion   int            `yaml:"api_version" json:"api_version"`
	ID           string         `yaml:"id" json:"id"`
	Name         string         `yaml:"name" json:"name"`
	Version      string         `yaml:"version" json:"version"`
	Runtime      Runtime        `yaml:"runtime" json:"runtime"`
	Provides     []string       `yaml:"provides" json:"provides"`
	Requires     Requirements   `yaml:"requires" json:"requires"`
	Capabilities []string       `yaml:"capabilities" json:"capabilities"`
	Permissions  Permissions    `yaml:"permissions" json:"permissions"`
	ConfigSchema map[string]any `yaml:"config_schema" json:"config_schema"`
	Path         string         `yaml:"-" json:"-"`
}
type Runtime struct {
	Type    string   `yaml:"type" json:"type"`
	Command string   `yaml:"command" json:"command"`
	Args    []string `yaml:"args,omitempty" json:"args,omitempty"`
}
type Requirements struct {
	Gateway string `yaml:"gateway" json:"gateway"`
}
type Permissions struct {
	Network []string `yaml:"network" json:"network"`
	Storage []string `yaml:"storage" json:"storage"`
	Secrets []string `yaml:"secrets" json:"secrets"`
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)+$`)
var capPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)
var hostPattern = regexp.MustCompile(`^(\*|(?:\*\.)?[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?)$`)
var secretPattern = regexp.MustCompile(`^env:[A-Za-z_][A-Za-z0-9_]*$`)

func (m Manifest) Validate() error {
	bad := func(s string) error { return fmt.Errorf("manifest %q: %s", m.ID, s) }
	if len(m.ID) > 100 || !idPattern.MatchString(m.ID) {
		return bad("invalid id")
	}
	if _, e := parseVersion(m.Version); e != nil {
		return bad(e.Error())
	}
	if m.APIVersion != sdk.APIVersion {
		return bad("unsupported api_version")
	}
	if m.Name == "" || len(m.Name) > 200 {
		return bad("invalid name")
	}
	if m.Runtime.Type != "process" {
		return bad("runtime.type must be process")
	}
	c := strings.ReplaceAll(m.Runtime.Command, "\\", "/")
	if !strings.HasPrefix(c, "./") || strings.Contains(c, ":") || strings.ContainsRune(c, 0) {
		return bad("command must be package-relative (./binary)")
	}
	for _, p := range strings.Split(c, "/") {
		if p == ".." {
			return bad("command escapes package")
		}
	}
	if c == "./" || len(m.Runtime.Args) > 64 {
		return bad("invalid runtime command/args")
	}
	if ok, e := Satisfies(sdk.GatewayVersion, m.Requires.Gateway); e != nil || !ok {
		return bad("incompatible or invalid requires.gateway")
	}
	known := map[string]bool{}
	for _, k := range []string{sdk.KindProtocol, sdk.KindUpstream, sdk.KindProvider, sdk.KindAuthenticator, sdk.KindScheduler, sdk.KindMiddleware, sdk.KindStorage, sdk.KindUsage, sdk.KindBilling, sdk.KindQuota, sdk.KindObservability, sdk.KindAdmin, sdk.KindSecretProvider} {
		known[k] = true
	}
	if len(m.Provides) == 0 {
		return bad("provides is required")
	}
	seen := map[string]bool{}
	for _, k := range m.Provides {
		if !known[k] || seen[k] {
			return bad("invalid/duplicate provides")
		}
		seen[k] = true
	}
	seen = map[string]bool{}
	for _, k := range m.Capabilities {
		if len(k) > 100 || !capPattern.MatchString(k) || seen[k] {
			return bad("invalid/duplicate capability")
		}
		seen[k] = true
	}
	for _, h := range m.Permissions.Network {
		if !hostPattern.MatchString(h) {
			return bad("invalid network permission")
		}
	}
	for _, s := range m.Permissions.Secrets {
		if !secretPattern.MatchString(s) || strings.HasPrefix(strings.ToUpper(s), "ENV:MODUAPI_") {
			return bad("invalid secret permission")
		}
	}
	for _, s := range m.Permissions.Storage {
		if s == "" || strings.Contains(s, "..") || strings.ContainsAny(s, "\\:") {
			return bad("invalid storage permission namespace")
		}
	}
	if m.ConfigSchema == nil || m.ConfigSchema["type"] != "object" {
		return bad("config_schema must describe an object")
	}
	if _, e := json.Marshal(m.ConfigSchema); e != nil {
		return bad("config_schema must be JSON compatible")
	}
	return nil
}
func Read(path string) (Manifest, error) {
	var m Manifest
	f, e := os.Open(path)
	if e != nil {
		return m, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if e != nil {
		return m, e
	}
	if len(b) > 256<<10 {
		return m, fmt.Errorf("manifest too large")
	}
	var node yaml.Node
	if e = yaml.Unmarshal(b, &node); e != nil {
		return m, e
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return m, fmt.Errorf("manifest must be a mapping")
	}
	keys := map[string]bool{}
	for i := 0; i < len(node.Content[0].Content); i += 2 {
		keys[node.Content[0].Content[i].Value] = true
	}
	for _, required := range []string{"id", "name", "version", "api_version", "runtime", "provides", "requires", "capabilities", "permissions", "config_schema"} {
		if !keys[required] {
			return m, fmt.Errorf("missing manifest field %s", required)
		}
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	if e = d.Decode(&m); e != nil {
		return m, e
	}
	var tail any
	if e = d.Decode(&tail); e != io.EOF {
		return m, fmt.Errorf("manifest must have one YAML document")
	}
	m.Path, e = filepath.Abs(path)
	if e != nil {
		return m, e
	}
	return m, m.Validate()
}
func Discover(dir string) ([]Manifest, error) {
	var result []Manifest
	ids := map[string]bool{}
	e := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported in plugin bundles: %s", path)
		}
		if d.Name() != "manifest.yaml" {
			return nil
		}
		m, e := Read(path)
		if e != nil {
			return fmt.Errorf("%s: %w", path, e)
		}
		if ids[m.ID] {
			return fmt.Errorf("duplicate plugin id %s", m.ID)
		}
		ids[m.ID] = true
		result = append(result, m)
		if len(result) > 128 {
			return fmt.Errorf("too many plugin manifests")
		}
		return nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, e
}
