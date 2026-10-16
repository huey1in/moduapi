// Package buildutil creates portable plugin bundles for development and tests.
package buildutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var Plugins = map[string]string{"protocol.openai": "protocol-openai", "upstream.http": "upstream-http", "scheduler.single": "scheduler-single", "scheduler.roundrobin": "scheduler-roundrobin"}

func Suffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
func Build(root, destination string, race bool) error {
	for id, folder := range Plugins {
		dir := filepath.Join(destination, id)
		if e := os.MkdirAll(dir, 0755); e != nil {
			return e
		}
		args := []string{"build", "-trimpath"}
		if race {
			args = append(args, "-race")
		}
		args = append(args, "-o", filepath.Join(dir, "plugin"+Suffix()), "./plugins/official/"+folder+"/cmd")
		cmd := exec.Command("go", args...)
		cmd.Dir = root
		if b, e := cmd.CombinedOutput(); e != nil {
			return fmt.Errorf("build %s: %w\n%s", id, e, b)
		}
		data, e := os.ReadFile(filepath.Join(root, "plugins", "official", folder, "manifest.yaml"))
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(dir, "manifest.yaml"), data, 0644); e != nil {
			return e
		}
	}
	return nil
}
