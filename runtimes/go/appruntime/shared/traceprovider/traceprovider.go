package traceprovider

import (
	"math/rand/v2"
	"strings"

	"encore.dev/appruntime/exported/trace2"
)

type Factory interface {
	NewLogger() trace2.Logger
	SampleTrace(service, endpoint string) bool
	SampleDefault() bool
}

// samplingRates holds pre-split sampling rates for fast lookup.
type samplingRates struct {
	// endpoint holds rates keyed by "service.endpoint" for API endpoints.
	endpoint map[string]float64
	// service holds rates keyed by service name.
	service map[string]float64
	// defaultRate is the default rate, or -1 if unset.
	defaultRate float64
}

type DefaultFactory struct {
	rates *samplingRates // nil means always sample
}

func NewDefaultFactory(cfg map[string]float64) *DefaultFactory {
	if len(cfg) == 0 {
		return &DefaultFactory{}
	}

	r := &samplingRates{defaultRate: -1}
	for key, rate := range cfg {
		switch {
		case key == "_":
			r.defaultRate = rate
		case strings.HasPrefix(key, "service:"):
			if r.service == nil {
				r.service = make(map[string]float64)
			}
			r.service[key[len("service:"):]] = rate
		case strings.HasPrefix(key, "endpoint:"):
			if r.endpoint == nil {
				r.endpoint = make(map[string]float64)
			}
			r.endpoint[key[len("endpoint:"):]] = rate
		}
	}
	return &DefaultFactory{rates: r}
}

func (f *DefaultFactory) NewLogger() trace2.Logger {
	return trace2.NewLog()
}

// SampleTrace determines whether to sample an API endpoint trace.
// Lookup order: endpoint → service → default.
func (f *DefaultFactory) SampleTrace(service, endpoint string) bool {
	r := f.rates
	if r == nil {
		return true
	}
	if len(r.endpoint) > 0 {
		if rate, ok := r.endpoint[service+"."+endpoint]; ok {
			return rand.Float64() < rate
		}
	}
	if rate, ok := r.service[service]; ok {
		return rand.Float64() < rate
	}
	if r.defaultRate >= 0 {
		return rand.Float64() < r.defaultRate
	}
	return true
}

// SampleDefault determines whether to sample based on the default sampling rate.
// If no default rate is configured, always samples.
func (f *DefaultFactory) SampleDefault() bool {
	r := f.rates
	if r == nil {
		return true
	}
	if r.defaultRate >= 0 {
		return rand.Float64() < r.defaultRate
	}
	return true
}
