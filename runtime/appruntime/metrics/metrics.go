package metrics

import (
	"net/http"
	"strings"

	"encore.dev/beta/errs"
)

var r = strings.NewReplacer(
	" ", "_",
	"-", "_",
	"'", "_",
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

	code := http.StatusText(httpStatus)
	if code == "" {
		code = http.StatusText(http.StatusOK)
	}
	return r.Replace(strings.ToLower(code))
}
