package metrics

import (
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

func (m *Manager) ReqEnd(service, endpoint string, err error, httpStatus int, durSecs float64) {
	code := code(err, httpStatus)
	m.exp.IncCounter(
		"e_requests_total",
		"service", service,
		"endpoint", endpoint,
		"code", code,
	)
	m.exp.Observe(
		"e_request_duration_seconds",
		"duration", durSecs,
		"service", service,
		"endpoint", endpoint,
		"code", code,
	)
}

func code(err error, httpStatus int) string {
	if err != nil {
		e := errs.Convert(err).(*errs.Error)
		return e.Code.String()
	}

	if httpStatus == 0 {
		return errs.OK.String()
	}

	if code := errs.HTTPStatusToCode(httpStatus); code != errs.Unknown {
		return code.String()
	}
	return "http_" + strconv.Itoa(httpStatus)
}
