package system

import (
	"runtime"
)

type SysMetrics struct {
	MemoryUsageBytes uint64
	NumGoRoutines    int
}

func ReadSysMetrics() SysMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return SysMetrics{
		MemoryUsageBytes: memStats.Alloc,
		NumGoRoutines:    runtime.NumGoroutine(),
	}
}
