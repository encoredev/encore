package metrics

type NullMetricsExporter struct{}

func NewNullMetricsExporter() *NullMetricsExporter {
	return &NullMetricsExporter{}
}

func (e *NullMetricsExporter) IncCounter(_ string, _ ...string) {
	return
}

func (e *NullMetricsExporter) Observe(_ string, _ string, _ float64, _ ...string) {
	return
}
