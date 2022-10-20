package metrics

type NullMetricsExporter struct{}

func NewNullMetricsExporter() *NullMetricsExporter {
	return &NullMetricsExporter{}
}

func (e *NullMetricsExporter) IncCounter(_ string, _ ...string) {
	return
}
