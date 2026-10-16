package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `server: {listen: "127.0.0.1:0"}
plugins: {directory: ./bundles}
routes:
  - {id: default, protocol: openai.chat, model: "*", upstreams: [a], scheduler: scheduler.single}
upstreams:
  a:
    plugin: upstream.http
    config: {base_url: "http://127.0.0.1:9090/v1", arbitrary_plugin_field: true}
`

func TestLoadStrictCoreOpaquePluginConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte(sample), 0644)
	c, e := Load(path)
	if e != nil {
		t.Fatal(e)
	}
	if c.Plugins.Directory != filepath.Join(dir, "bundles") || c.Routes[0].Retry.MaxAttempts != 1 {
		t.Fatalf("defaults/relative path: %+v", c)
	}
	if c.Upstreams["a"].Config["arbitrary_plugin_field"] != true {
		t.Fatal("plugin config lost")
	}
	for _, extra := range []string{"\nunknown_core_field: true\n", "\n---\nserver: {}\n"} {
		_ = os.WriteFile(path, []byte(sample+extra), 0644)
		if _, e := Load(path); e == nil {
			t.Fatal("invalid core config accepted")
		}
	}
}
