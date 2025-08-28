//go:build encore_app

package api_test

import (
	"context"
	"fmt"
	"testing"

	"encore.dev/appruntime/apisdk/api"
	"encr.dev/pkg/option"
)

// MockHandler implements the Handler interface for testing
type MockHandler struct {
	serviceName  string
	endpointName string
	accessType   api.Access
	semanticPath string
	routerPath   string
	methods      []string
	isFallback   bool
}

func (h *MockHandler) ServiceName() string    { return h.serviceName }
func (h *MockHandler) EndpointName() string  { return h.endpointName }
func (h *MockHandler) AccessType() api.Access { return h.accessType }
func (h *MockHandler) SemanticPath() string  { return h.semanticPath }
func (h *MockHandler) HTTPRouterPath() string { return h.routerPath }
func (h *MockHandler) HTTPMethods() []string  { return h.methods }
func (h *MockHandler) IsFallback() bool       { return h.isFallback }
func (h *MockHandler) Handle(c api.IncomingContext) {}

func ExampleLookupEndpoint() {
	// This example demonstrates how to use the LookupEndpoint function
	// to find a registered endpoint by service and endpoint name.

	// First, register a mock endpoint
	handler := &MockHandler{
		serviceName:  "user",
		endpointName: "GetUser",
		accessType:   api.Public,
		semanticPath: "/user/:id",
		routerPath:   "/user/:id",
		methods:      []string{"GET"},
		isFallback:   false,
	}
	
	// Register the endpoint
	api.RegisterEndpoint(handler, func(ctx context.Context, id string) error { return nil })
	
	// Now lookup the endpoint
	result := api.LookupEndpoint("user", "GetUser")
	
	if foundHandler, ok := result.Get(); ok {
		fmt.Printf("Found endpoint: %s.%s\n", foundHandler.ServiceName(), foundHandler.EndpointName())
		fmt.Printf("Access type: %s\n", foundHandler.AccessType())
		fmt.Printf("HTTP methods: %v\n", foundHandler.HTTPMethods())
	} else {
		fmt.Println("Endpoint not found")
	}
	
	// Try to lookup a non-existent endpoint
	notFound := api.LookupEndpoint("user", "NonExistentEndpoint")
	if _, ok := notFound.Get(); !ok {
		fmt.Println("NonExistentEndpoint was correctly not found")
	}
	
	// Output:
	// Found endpoint: user.GetUser
	// Access type: public
	// HTTP methods: [GET]
	// NonExistentEndpoint was correctly not found
}

func TestLookupEndpoint(t *testing.T) {
	// Create a mock handler
	handler := &MockHandler{
		serviceName:  "test",
		endpointName: "TestEndpoint",
		accessType:   api.Public,
		semanticPath: "/test",
		routerPath:   "/test",
		methods:      []string{"POST"},
		isFallback:   false,
	}
	
	// Register the endpoint
	api.RegisterEndpoint(handler, func(ctx context.Context) error { return nil })
	
	// Test successful lookup
	result := api.LookupEndpoint("test", "TestEndpoint")
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
	notFound := api.LookupEndpoint("nonexistent", "TestEndpoint")
	if _, ok := notFound.Get(); ok {
		t.Error("Expected not to find endpoint in non-existent service")
	}
	
	// Test lookup of non-existent endpoint in existing service
	notFound = api.LookupEndpoint("test", "NonExistentEndpoint")
	if _, ok := notFound.Get(); ok {
		t.Error("Expected not to find non-existent endpoint")
	}
}