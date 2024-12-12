package traceprovider

import (
	"math/rand/v2"

	"encore.dev/appruntime/exported/trace2"
)

type Factory interface {
	NewLogger() trace2.Logger
	SampleTrace() bool
}

type DefaultFactory struct {
	// SampleRate is the rate at which to sample traces, between [0, 1].
	// If nil, 100% of traces are sampled.
	SampleRate *float64
}

func (f *DefaultFactory) NewLogger() trace2.Logger {
	return trace2.NewLog()
}

func (f *DefaultFactory) SampleTrace() bool {
	if f.SampleRate == nil {
		return true
	} else {
		sample := rand.Float64() < *f.SampleRate
		return sample
	}
}
