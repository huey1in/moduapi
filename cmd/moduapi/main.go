package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/huey1in/moduapi/core/config"
	"github.com/huey1in/moduapi/internal/app"
	"github.com/huey1in/moduapi/runtime/manifest"
	"github.com/huey1in/moduapi/sdk"
)

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "moduapi:", e)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: moduapi serve --config config.yaml | plugin list/inspect/validate | version")
	}
	switch args[0] {
	case "version":
		fmt.Println(sdk.GatewayVersion)
		return nil
	case "serve":
		return serve(args[1:])
	case "plugin":
		return plugins(args[1:])
	default:
		return fmt.Errorf("unknown command %s", args[0])
	}
}
func serve(args []string) error {
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	path := f.String("config", "config.yaml", "kernel configuration")
	if e := f.Parse(args); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	c, e := config.Load(*path)
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	lis, e := net.Listen("tcp", c.Server.Listen)
	if e != nil {
		return e
	}
	defer lis.Close()
	a, e := app.Start(ctx, c, nil)
	if e != nil {
		return e
	}
	defer a.Manager.Close(context.Background())
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := &http.Server{Handler: a.Gateway, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 << 10, BaseContext: func(net.Listener) context.Context { return base }}
	result := make(chan error, 1)
	go func() { result <- srv.Serve(lis) }()
	fmt.Fprintln(os.Stderr, "ModuAPI", sdk.GatewayVersion, "listening on", lis.Addr())
	select {
	case e := <-result:
		if e != http.ErrServerClosed {
			return e
		}
		return nil
	case <-ctx.Done():
	}
	drain, finish := context.WithTimeout(context.Background(), 10*time.Second)
	defer finish()
	if e = srv.Shutdown(drain); e != nil {
		cancel()
		_ = srv.Close()
	}
	<-result
	return nil
}
func plugins(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: moduapi plugin list --directory dist/plugins | inspect/validate <manifest-or-directory>")
	}
	if args[0] == "list" {
		f := flag.NewFlagSet("plugin list", flag.ContinueOnError)
		dir := f.String("directory", "dist/plugins", "plugin bundle directory")
		if e := f.Parse(args[1:]); e != nil {
			return e
		}
		list, e := manifest.Discover(*dir)
		if e != nil {
			return e
		}
		for _, m := range list {
			fmt.Printf("%s\t%s\t%s\n", m.ID, m.Version, strings.Join(m.Provides, ","))
		}
		return nil
	}
	if (args[0] != "inspect" && args[0] != "validate") || len(args) != 2 {
		return fmt.Errorf("expected inspect/validate and one manifest path")
	}
	path := args[1]
	if st, e := os.Stat(path); e == nil && st.IsDir() {
		path = filepath.Join(path, "manifest.yaml")
	}
	m, e := manifest.Read(path)
	if e != nil {
		return e
	}
	if args[0] == "validate" {
		fmt.Printf("valid: %s %s (API %d)\n", m.ID, m.Version, m.APIVersion)
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}
