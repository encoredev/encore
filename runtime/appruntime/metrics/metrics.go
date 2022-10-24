package metrics

import (
	"net/http"
	"strconv"

	"encore.dev/beta/errs"
)

type Manager struct {
	exp Exporter
}

type Exporter interface {
	IncCounter(name string, tags ...string)
	Observe(name string, key string, value float64, tags ...string)
}

func NewManager(exp Exporter) *Manager {
	return &Manager{exp: exp}
}

func (m *Manager) ReqBegin(service, endpoint string) {
	m.exp.IncCounter("e_requests_total", "service", service, "endpoint", endpoint)
}

func (m *Manager) ReqEnd(service, endpoint string, err error, httpStatus int, durMillis int64) {
	if err != nil {
		e := errs.Convert(err).(*errs.Error)
		m.exp.IncCounter("e_errors_total", "service", service, "endpoint", endpoint, "code", e.Code.String())

		if httpStatus == 0 {
			httpStatus = e.Code.HTTPStatus()
		}
	}

	if httpStatus == 0 {
		httpStatus = http.StatusOK
	}
	m.exp.Observe(
		"e_request_durations_milliseconds",
		"duration", float64(durMillis),
		"service", service,
		"endpoint", endpoint,
		"status_code", strconv.Itoa(httpStatus),
	)
}

func (m *Manager) UnknownEndpoint(service, endpoint string) {
	m.exp.IncCounter("e_requests_unknown_endpoint_total", "service", service, "endpoint", endpoint)
}
