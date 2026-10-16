//go:build !linux && !windows

package main

import "fmt"

const descriptorKind = "unsupported"

func readProcess(int) (resources, error) {
	return resources{}, fmt.Errorf("resource sampling requires Linux or Windows")
}
func cpuName() string { return "unknown" }
