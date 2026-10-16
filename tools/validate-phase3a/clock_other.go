//go:build !windows

package main

import "time"

var clockOrigin = time.Now()

func nanoNow() int64 { return time.Since(clockOrigin).Nanoseconds() }
