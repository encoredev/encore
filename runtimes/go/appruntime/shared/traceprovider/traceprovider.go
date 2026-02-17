package traceprovider

import (
	"math/rand/v2"

	"encore.dev/appruntime/exported/trace2"
)

type Factory interface {
	NewLogger() trace2.Logger
	SampleTrace(service, endpoint string) bool
}

type DefaultFactory struct {
	// SamplingConfig holds sampling rates keyed by:
	//   "service.endpoint" - endpoint-level rate
	//   "service"          - service-level rate
	//   "_"                - global default rate
	// Values are between [0, 1]. If no match is found, all traces are sampled.
	SamplingConfig map[string]float64
}

func (f *DefaultFactory) NewLogger() trace2.Logger {
	return trace2.NewLog()
}

func (f *DefaultFactory) SampleTrace(service, endpoint string) bool {
	if len(f.SamplingConfig) == 0 {
		return true
	}

	// Look up by "service.endpoint", then "service", then "_".
	key := service + "." + endpoint
	if rate, ok := f.SamplingConfig[key]; ok {
		return rand.Float64() < rate
	}
	if rate, ok := f.SamplingConfig[service]; ok {
		return rand.Float64() < rate
	}
	if rate, ok := f.SamplingConfig["_"]; ok {
		return rand.Float64() < rate
	}
	return true
}
