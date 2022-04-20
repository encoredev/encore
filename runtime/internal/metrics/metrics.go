package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func Gather() ([]*dto.MetricFamily, error) {
	return prometheus.DefaultGatherer.Gather()
}

func ReqBegin(service, api string) {
	rpcCountTotal.Add(1)
	rpcCount.WithLabelValues(service, api).Add(1)
}

func ReqEnd(service, api string, durSecs float64, code string) {
	rpcDuration.WithLabelValues(service, api, code).Observe(durSecs)
}

func UnknownEndpoint(service, api string) {
	unknownEndpoint.WithLabelValues(service, api).Add(1)
}

func init() {
	prometheus.MustRegister(rpcCountTotal, rpcCount, rpcDuration, unknownEndpoint)
}

var (
	rpcCountTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rpc_count_total",
		Help: "Total RPC count",
	})

	rpcCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rpc_count_endpoint_total",
		Help: "Per-endpoint RPC counts",
	}, []string{"service", "api"})

	rpcDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rpc_durations_histogram_seconds",
		Help:    "RPC latency distributions.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "api", "status"})

	unknownEndpoint = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rpc_unknown_endpoint_total",
		Help: "RPC calls to unknown endpoints",
	}, []string{"service", "api"})
)
