package main

import (
	"flag"
	"fmt"
	"github.com/huey1in/moduapi/internal/mockupstream"
	"net/http"
	"os"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9090", "listen address")
	flag.Parse()
	s := &http.Server{Addr: *listen, Handler: http.HandlerFunc(mockupstream.Handler), ReadHeaderTimeout: 5 * time.Second}
	fmt.Fprintln(os.Stderr, "mock upstream listening on", *listen)
	if e := s.ListenAndServe(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
