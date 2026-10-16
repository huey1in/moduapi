//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const descriptorKind = "Windows handles (not POSIX file descriptors)"

var psapi = syscall.NewLazyDLL("psapi.dll")
var memoryInfo = psapi.NewProc("GetProcessMemoryInfo")
var handleCount = syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessHandleCount")
var queryCounter = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryPerformanceCounter")
var queryFrequency = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryPerformanceFrequency")
var frequency = func() int64 {
	var v int64
	ok, _, _ := queryFrequency.Call(uintptr(unsafe.Pointer(&v)))
	if ok == 0 || v <= 0 {
		panic("QueryPerformanceFrequency unavailable")
	}
	return v
}()

// Go's shared interrupt clock can quantize short Windows measurements. Only
// this harness uses QPC; the production gateway's clock/deadlines are unchanged.
func nanoNow() int64 {
	var v int64
	ok, _, _ := queryCounter.Call(uintptr(unsafe.Pointer(&v)))
	if ok == 0 {
		panic("QueryPerformanceCounter unavailable")
	}
	return (v/frequency)*1e9 + (v%frequency)*1e9/frequency
}

type memoryCounters struct {
	Size, Faults                                                                                 uint32
	PeakWorkingSet, WorkingSet, PeakPaged, Paged, PeakNonPaged, NonPaged, Pagefile, PeakPagefile uintptr
}

func readProcess(pid int) (resources, error) {
	h, e := syscall.OpenProcess(0x0400|0x0010, false, uint32(pid))
	if e != nil {
		return resources{}, e
	}
	defer syscall.CloseHandle(h)
	var created, exit, kernel, user syscall.Filetime
	if e = syscall.GetProcessTimes(h, &created, &exit, &kernel, &user); e != nil {
		return resources{}, e
	}
	ticks := func(t syscall.Filetime) uint64 { return uint64(t.HighDateTime)<<32 | uint64(t.LowDateTime) }
	m := memoryCounters{}
	m.Size = uint32(unsafe.Sizeof(m))
	ok, _, e := memoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&m)), uintptr(m.Size))
	if ok == 0 {
		return resources{}, fmt.Errorf("memory: %w", e)
	}
	var count uint32
	ok, _, e = handleCount.Call(uintptr(h), uintptr(unsafe.Pointer(&count)))
	if ok == 0 {
		return resources{}, fmt.Errorf("handles: %w", e)
	}
	return resources{float64(ticks(kernel)+ticks(user)) / 1e7, uint64(m.WorkingSet), uint64(count)}, nil
}
func cpuName() string { return os.Getenv("PROCESSOR_IDENTIFIER") }
