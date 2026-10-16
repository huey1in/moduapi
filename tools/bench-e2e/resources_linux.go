//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const descriptorKind = "POSIX file descriptors"

var clockTicks = func() float64 {
	b, e := exec.Command("getconf", "CLK_TCK").Output()
	if e != nil {
		panic(e)
	}
	v, e := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if e != nil || v <= 0 {
		panic("invalid CLK_TCK")
	}
	return v
}()

func readProcess(pid int) (resources, error) {
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if e != nil {
		return resources{}, e
	}
	f := strings.Fields(string(b)[strings.LastIndexByte(string(b), ')')+1:])
	u, e := strconv.ParseFloat(f[11], 64)
	if e != nil {
		return resources{}, e
	}
	s, e := strconv.ParseFloat(f[12], 64)
	if e != nil {
		return resources{}, e
	}
	rss, e := strconv.ParseUint(f[21], 10, 64)
	if e != nil {
		return resources{}, e
	}
	fds, e := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if e != nil {
		return resources{}, e
	}
	return resources{(u + s) / clockTicks, rss * uint64(os.Getpagesize()), uint64(len(fds))}, nil
}
func cpuName() string {
	b, _ := os.ReadFile("/proc/cpuinfo")
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "model name") {
			return strings.TrimSpace(strings.SplitN(l, ":", 2)[1])
		}
	}
	return "unknown"
}
