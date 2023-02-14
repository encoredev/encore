package system

import (
	"runtime"
)

const (
	MetricNameMemUsageBytes = "e_memory_usage_bytes"
	MetricNameNumGoroutines = "e_num_goroutines"
)

func ReadSysMetrics() map[string]uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return map[string]uint64{
		MetricNameMemUsageBytes: memStats.Alloc,
		MetricNameNumGoroutines: uint64(runtime.NumGoroutine()),
	}
}
