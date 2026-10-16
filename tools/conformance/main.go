// Sets up a private, pinned Python environment. Ordinary Go tests never run this.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func run(name string, args ...string) {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	env := filepath.Join(root, ".venv-conformance")
	python := filepath.Join(env, "bin", "python")
	if runtime.GOOS == "windows" {
		python = filepath.Join(env, "Scripts", "python.exe")
	}
	if _, err := os.Stat(python); err != nil {
		run("python", "-m", "venv", env)
	}
	run(python, "-m", "pip", "install", "--quiet", "--disable-pip-version-check", "-r", "tests/conformance/python/requirements.txt")
	out := filepath.Join(root, "tests", "conformance", "python")
	run(python, "-m", "grpc_tools.protoc", "-Iproto", "--python_out="+out, "--grpc_python_out="+out, "proto/moduapi/plugin/v1/plugin.proto")
	os.Setenv("MODUAPI_CONFORMANCE_PYTHON", python)
	run("go", "test", "-tags=conformance", "-count=1", "-race", "./tests/conformance/...")
}
