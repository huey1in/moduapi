package main

import (
	"flag"
	"fmt"
	"github.com/huey1in/moduapi/internal/buildutil"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	race := flag.Bool("race", false, "instrument plugins and host")
	flag.Parse()
	root, e := os.Getwd()
	if e == nil {
		e = buildutil.Build(root, filepath.Join(root, "dist", "plugins"), *race)
	}
	if e == nil {
		args := []string{"build", "-trimpath"}
		if *race {
			args = append(args, "-race")
		}
		args = append(args, "-o", filepath.Join(root, "dist", "moduapi"+buildutil.Suffix()), "./cmd/moduapi")
		cmd := exec.Command("go", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		e = cmd.Run()
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
