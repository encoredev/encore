package metrics

type Manager struct {
	exp Exporter
}

type Exporter interface {
	IncCounter(name string, tags ...string)
}

func NewManager(exp Exporter) *Manager {
	return &Manager{exp: exp}
}

func (m *Manager) ReqBegin(service, endpoint string) {
	m.exp.IncCounter("e_requests_total", "service", service, "endpoint", endpoint)
}

func (m *Manager) ReqEnd(service, endpoint, code string, durSecs float64) {
	// TODO
}

func (m *Manager) UnknownEndpoint(service, endpoint string) {
	m.exp.IncCounter("e_requests_unknown_endpoint_total", "service", service, "endpoint", endpoint)
}
