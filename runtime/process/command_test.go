package process

import (
	"github.com/huey1in/moduapi/runtime/manifest"
	"os"
	"path/filepath"
	"testing"
)

func TestPackageCommandBoundary(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "plugin.py")
	if e := os.WriteFile(script, []byte("pass"), 0600); e != nil {
		t.Fatal(e)
	}
	m := manifest.Manifest{Path: filepath.Join(root, "manifest.yaml"), Runtime: manifest.Runtime{Type: "interpreter", Interpreter: "python", Command: "plugin.py"}}
	got, e := executable(m)
	if e != nil || got != script {
		t.Fatalf("%s %v", got, e)
	}
	outside := filepath.Join(t.TempDir(), "outside.py")
	_ = os.WriteFile(outside, []byte("pass"), 0600)
	link := filepath.Join(root, "escape.py")
	if e := os.Symlink(outside, link); e != nil {
		t.Skip("symlink privilege unavailable")
	}
	m.Runtime.Command = "escape.py"
	if _, e = executable(m); e == nil {
		t.Fatal("symlink escaped package")
	}
}
