package api

import (
	"testing"

	"encr.dev/pkg/option"
)

// MockHandler implements the Handler interface for testing
type MockHandler struct {
	serviceName  string
	endpointName string
	accessType   Access
	semanticPath string
	routerPath   string
	methods      []string
	isFallback   bool
}

func (h *MockHandler) ServiceName() string     { return h.serviceName }
func (h *MockHandler) EndpointName() string   { return h.endpointName }
func (h *MockHandler) AccessType() Access     { return h.accessType }
func (h *MockHandler) SemanticPath() string   { return h.semanticPath }
func (h *MockHandler) HTTPRouterPath() string { return h.routerPath }
func (h *MockHandler) HTTPMethods() []string  { return h.methods }
func (h *MockHandler) IsFallback() bool        { return h.isFallback }
func (h *MockHandler) Handle(c IncomingContext) {}

func TestServerLookupEndpoint(t *testing.T) {
	// Create a test server
	server := &Server{
		endpointLookup: make(map[string]map[string]Handler),
	}
	
	// Create a mock handler
	handler := &MockHandler{
		serviceName:  "test",
		endpointName: "TestEndpoint",
		accessType:   Public,
		semanticPath: "/test",
		routerPath:   "/test",
		methods:      []string{"POST"},
		isFallback:   false,
	}
	
	// Manually add to lookup table
	if server.endpointLookup["test"] == nil {
		server.endpointLookup["test"] = make(map[string]Handler)
	}
	server.endpointLookup["test"]["TestEndpoint"] = handler
	
	// Test successful lookup
	result := server.LookupEndpoint("test", "TestEndpoint")
	if foundHandler, ok := result.Get(); ok {
		if foundHandler.ServiceName() != "test" {
			t.Errorf("Expected service name 'test', got '%s'", foundHandler.ServiceName())
		}
		if foundHandler.EndpointName() != "TestEndpoint" {
			t.Errorf("Expected endpoint name 'TestEndpoint', got '%s'", foundHandler.EndpointName())
		}
	} else {
		t.Error("Expected to find the registered endpoint")
	}
	
	// Test lookup of non-existent service
	notFound := server.LookupEndpoint("nonexistent", "TestEndpoint")
	if _, ok := notFound.Get(); ok {
		t.Error("Expected not to find endpoint in non-existent service")
	}
	
	// Test lookup of non-existent endpoint in existing service
	notFound = server.LookupEndpoint("test", "NonExistentEndpoint")
	if _, ok := notFound.Get(); ok {
		t.Error("Expected not to find non-existent endpoint")
	}
	
	// Test that result is properly typed as option.Option[Handler]
	var _ option.Option[Handler] = result
}