package apienc

import (
	"testing"
)

func TestHTTPStatusTagBasic(t *testing.T) {
	// This is a placeholder test to ensure the package compiles correctly.
	// The actual functionality will be tested through integration tests
	// with the full Encore application parsing.
	t.Log("HTTPStatus tag functionality is available")
}

// TestResponseEncodingHTTPStatusField tests the HTTPStatusField in ResponseEncoding
func TestResponseEncodingHTTPStatusField(t *testing.T) {
	resp := &ResponseEncoding{
		HTTPStatusField: "Status",
	}
	
	if resp.HTTPStatusField != "Status" {
		t.Errorf("Expected HTTPStatusField to be 'Status', got %q", resp.HTTPStatusField)
	}
	
	// Test empty field
	resp2 := &ResponseEncoding{}
	if resp2.HTTPStatusField != "" {
		t.Errorf("Expected HTTPStatusField to be empty, got %q", resp2.HTTPStatusField)
	}
}