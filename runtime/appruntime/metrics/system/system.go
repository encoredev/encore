package system

import (
	"runtime/metrics"

	"github.com/rs/zerolog"
)

// These are the metrics exposed by Go we currently track. Once we update our
// fork to Go 1.20, we should add:
//
// - /cpu/classes/gc/pause:cpu-seconds
// - /cpu/classes/idle:cpu-seconds
// - /cpu/classes/total:cpu-seconds
// - /sync/mutex/wait/total:seconds
const (
	MetricNameHeapObjectsBytes = "e_sys_memory_heap_objects_bytes"
	MetricNameOSStacksBytes    = "e_sys_memory_os_stacks_bytes"
	MetricNameGoroutines       = "e_sys_sched_goroutines"

	goMetricHeapObjectsBytes = "/memory/classes/heap/objects:bytes"
	goMetricOSStacksBytes    = "/memory/classes/os-stacks:bytes"
	goMetricGoroutines       = "/sched/goroutines:goroutines"
)

var encoreMetricNames = map[string]string{
	goMetricHeapObjectsBytes: MetricNameHeapObjectsBytes,
	goMetricOSStacksBytes:    MetricNameOSStacksBytes,
	goMetricGoroutines:       MetricNameGoroutines,
}

func ReadSysMetrics(logger zerolog.Logger) map[string]uint64 {
	samples := []metrics.Sample{
		{Name: goMetricHeapObjectsBytes},
		{Name: goMetricOSStacksBytes},
		{Name: goMetricGoroutines},
	}
	metrics.Read(samples)

	output := make(map[string]uint64, len(samples))
	for _, sample := range samples {
		switch sample.Value.Kind() {
		case metrics.KindUint64:
			output[encoreMetricNames[sample.Name]] = sample.Value.Uint64()
		case metrics.KindBad:
			// This means the metric is unsupported. It's expected to happen very rarely
			// possibly due to a large change in a particular Go implementation.
			logger.Warn().Str("metric", sample.Name).Msg("metric no longer supported")
		default:
			logger.Warn().Str("metric", sample.Name).Msg("unexpected metric kind")
		}
	}
	return output
}
