package traceprovider

import (
	"math/rand/v2"
	"strings"

	"encore.dev/appruntime/exported/trace2"
)

type Factory interface {
	NewLogger() trace2.Logger
	SampleTrace(service, endpoint string) bool
}

// samplingRates holds pre-split sampling rates for fast lookup.
type samplingRates struct {
	// endpoint holds rates keyed by "service.endpoint".
	endpoint map[string]float64
	// service holds rates keyed by "service".
	service map[string]float64
	// global is the default rate (key "_"), or -1 if unset.
	global float64
}

type DefaultFactory struct {
	rates *samplingRates // nil means always sample
}

func NewDefaultFactory(config map[string]float64) *DefaultFactory {
	if len(config) == 0 {
		return &DefaultFactory{}
	}

	r := &samplingRates{global: -1}
	for key, rate := range config {
		switch {
		case key == "_":
			r.global = rate
		case strings.ContainsRune(key, '.'):
			if r.endpoint == nil {
				r.endpoint = make(map[string]float64)
			}
			r.endpoint[key] = rate
		default:
			if r.service == nil {
				r.service = make(map[string]float64)
			}
			r.service[key] = rate
		}
	}
	return &DefaultFactory{rates: r}
}

func (f *DefaultFactory) NewLogger() trace2.Logger {
	return trace2.NewLog()
}

func (f *DefaultFactory) SampleTrace(service, endpoint string) bool {
	r := f.rates
	if r == nil {
		return true
	}

	// Look up by "service.endpoint", then "service", then "_".
	if len(r.endpoint) > 0 {
		if rate, ok := r.endpoint[service+"."+endpoint]; ok {
			return rand.Float64() < rate
		}
	}
	if rate, ok := r.service[service]; ok {
		return rand.Float64() < rate
	}
	if r.global >= 0 {
		return rand.Float64() < r.global
	}
	return true
}
