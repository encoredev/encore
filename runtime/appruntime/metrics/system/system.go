package system

import (
	"runtime/metrics"

	"github.com/rs/zerolog"
)

type MetricKind int

const (
	MetricKindCounter MetricKind = iota
	MetricKindGauge

	MetricNameHeapObjectsBytes = "e_sys_memory_heap_objects_bytes"
	MetricNameGoroutines       = "e_sys_sched_goroutines"
	MetricNameTotalCPUSeconds  = "e_sys_cpu_total_cpuseconds"

	goMetricHeapObjectsBytes = "/memory/classes/heap/objects:bytes"
	goMetricGoroutines       = "/sched/goroutines:goroutines"
	goMetricTotalCPUSeconds  = "/cpu/classes/total:cpu-seconds"
)

type metadata struct {
	name string
	kind MetricKind
}

var encoreMetricMetadata = map[string]metadata{
	goMetricHeapObjectsBytes: {
		name: MetricNameHeapObjectsBytes,
		kind: MetricKindGauge,
	},
	goMetricGoroutines: {
		name: MetricNameGoroutines,
		kind: MetricKindGauge,
	},
	goMetricTotalCPUSeconds: {
		name: MetricNameTotalCPUSeconds,
		kind: MetricKindCounter,
	},
}

type SysMetric struct {
	Sample     metrics.Sample
	EncoreName string
	Kind       MetricKind
}

func ReadSysMetrics(logger zerolog.Logger) []*SysMetric {
	samples := []metrics.Sample{
		{Name: goMetricHeapObjectsBytes},
		{Name: goMetricGoroutines},
		{Name: goMetricTotalCPUSeconds},
	}
	metrics.Read(samples)

	output := make([]*SysMetric, 0, len(samples))
	for _, sample := range samples {
		if sample.Value.Kind() == metrics.KindBad {
			// This means the metric is unsupported. It's expected to happen very rarely
			// possibly due to a large change in a particular Go implementation.
			logger.Warn().Str("metric", sample.Name).Msg("metric no longer supported")
		} else {
			meta := encoreMetricMetadata[sample.Name]
			output = append(output, &SysMetric{
				Sample:     sample,
				EncoreName: meta.name,
				Kind:       meta.kind,
			})
		}
	}
	return output
}
