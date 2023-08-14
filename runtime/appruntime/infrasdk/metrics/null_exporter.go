package metrics

import (
	"context"

	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/metrics"
)

type NullMetricsExporter struct{}

func NewNullMetricsExporter() *NullMetricsExporter {
	return &NullMetricsExporter{}
}

func (e *NullMetricsExporter) Export(ctx context.Context, metrics []metrics.CollectedMetric) error {
	return nil
}

func (e *NullMetricsExporter) Shutdown(p *shutdown.Process) error { return nil }
