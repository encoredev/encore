package mock_trace

import "encore.dev/appruntime/trace"

func NewMockFactory(log *MockLogger) trace.Factory {
	return &mockFactory{log}
}

type mockFactory struct {
	log *MockLogger
}

func (f *mockFactory) NewLogger() trace.Logger {
	return f.log
}
