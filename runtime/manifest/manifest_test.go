package manifest

import (
	"github.com/huey1in/moduapi/sdk"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func valid() Manifest {
	return Manifest{ID: "test.plugin", Name: "Test", Version: "1.2.3", APIVersion: sdk.APIVersion, Runtime: Runtime{Type: "process", Command: "./plugin"}, Provides: []string{"scheduler"}, Requires: Requirements{Gateway: ">=0.1.0 <1.0.0"}, ConfigSchema: map[string]any{"type": "object"}}
}
func TestManifestValidation(t *testing.T) {
	if e := valid().Validate(); e != nil {
		t.Fatal(e)
	}
	for name, mutate := range map[string]func(*Manifest){"id": func(m *Manifest) { m.ID = "../escape" }, "version": func(m *Manifest) { m.Version = "01.2.3" }, "API": func(m *Manifest) { m.APIVersion = 2 }, "command": func(m *Manifest) { m.Runtime.Command = "./../plugin" }, "absolute": func(m *Manifest) { m.Runtime.Command = "C:/plugin.exe" }, "gateway": func(m *Manifest) { m.Requires.Gateway = ">=1.0.0" }, "capability": func(m *Manifest) { m.Capabilities = []string{"a", "a"} }, "schema": func(m *Manifest) { m.ConfigSchema = nil }, "network": func(m *Manifest) { m.Permissions.Network = []string{"https://example.com"} }, "secret": func(m *Manifest) { m.Permissions.Secrets = []string{"env:MODUAPI_PLUGIN_TOKEN"} }, "role": func(m *Manifest) { m.Provides = []string{"arbitrary"} }} {
		t.Run(name, func(t *testing.T) {
			m := valid()
			mutate(&m)
			if m.Validate() == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}
func TestSemanticVersions(t *testing.T) {
	for _, tc := range []struct {
		v, r string
		want bool
	}{
		{"0.1.0", ">=0.1.0 <1.0.0", true}, {"1.0.0", ">=0.1.0 <1.0.0", false}, {"1.2.3-rc.1", "<1.2.3", true}, {"1.2.3-alpha.9", "<1.2.3-alpha.10", true}, {"1.2.3-9", "<1.2.3-a", true}, {"1.2.3+build.1", "=1.2.3", true}} {
		ok, e := Satisfies(tc.v, tc.r)
		if e != nil || ok != tc.want {
			t.Fatalf("%+v: %v %v", tc, ok, e)
		}
	}
	for _, s := range []string{"v1.0.0", "1.0", "1.0.0-01", "1.0.0+", "1.0.0\n", "1.0.0-", "1.02.0"} {
		if _, e := parseVersion(s); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	for _, s := range []string{"", ">= 0.1.0", "^0.1.0", "*", "0.1.0 || 1.0.0"} {
		if _, e := Satisfies("0.1.0", s); e == nil {
			t.Fatalf("accepted range %q", s)
		}
	}
}

func TestInterpreterDeclaration(t *testing.T) {
	for _, command := range []string{"plugin.py", "./plugin.py", "scripts/main.py"} {
		m := valid()
		m.Runtime = Runtime{Type: "interpreter", Interpreter: "python", Command: command}
		if e := m.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	for _, command := range []string{"../plugin.py", "/plugin.py", "C:/plugin.py", "./a/../../plugin.py"} {
		m := valid()
		m.Runtime = Runtime{Type: "interpreter", Interpreter: "python", Command: command}
		if m.Validate() == nil {
			t.Fatal(command)
		}
	}
	for _, interpreter := range []string{"", "./python", "../python", "python -c"} {
		m := valid()
		m.Runtime = Runtime{Type: "interpreter", Interpreter: interpreter, Command: "plugin.py"}
		if m.Validate() == nil {
			t.Fatal(interpreter)
		}
	}
}
func TestDiscoveryAndStrictYAML(t *testing.T) {
	root := t.TempDir()
	source, e := os.ReadFile(filepath.Join("..", "..", "plugins", "official", "scheduler-single", "manifest.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	write := func(name string, b []byte) {
		dir := filepath.Join(root, name)
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, "manifest.yaml"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	write("a", source)
	list, e := Discover(root)
	if e != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, e)
	}
	write("b", source)
	if _, e = Discover(root); e == nil {
		t.Fatal("duplicate ID accepted")
	}
	for _, data := range []string{string(source) + "\nunknown_field: true\n", string(source) + "\n---\nid: extra\n", strings.Repeat("x", (256<<10)+1)} {
		path := filepath.Join(t.TempDir(), "manifest.yaml")
		_ = os.WriteFile(path, []byte(data), 0644)
		if _, e = Read(path); e == nil {
			t.Fatal("invalid YAML accepted")
		}
	}
}
