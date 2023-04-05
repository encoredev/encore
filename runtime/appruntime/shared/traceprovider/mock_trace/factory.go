package mock_trace

import (
	"encore.dev/appruntime/exported/trace"
	"encore.dev/appruntime/shared/traceprovider"
)

func NewMockFactory(log *MockLogger) traceprovider.Factory {
	return &mockFactory{log}
}

type mockFactory struct {
	log *MockLogger
}

func (f *mockFactory) NewLogger() trace.Logger {
	return f.log
}
