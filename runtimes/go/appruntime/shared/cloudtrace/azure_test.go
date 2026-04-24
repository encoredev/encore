package cloudtrace

import (
	"net/http/httptest"
	"testing"
)

// TestAzureConnectionStringFromEnv tests the private azureConnectionStringFromEnv helper
func TestAzureConnectionStringFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "empty environment",
			envVars:  map[string]string{},
			expected: "",
		},
		{
			name: "uppercase env var set",
			envVars: map[string]string{
				"APPLICATIONINSIGHTS_CONNECTION_STRING": "InstrumentationKey=abc123;IngestionEndpoint=https://example.com",
			},
			expected: "InstrumentationKey=abc123;IngestionEndpoint=https://example.com",
		},
		{
			name: "lowercase env var set",
			envVars: map[string]string{
				"applicationinsights_connection_string": "InstrumentationKey=xyz789;IngestionEndpoint=https://example.com",
			},
			expected: "InstrumentationKey=xyz789;IngestionEndpoint=https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			result := azureConnectionStringFromEnv()
			if result != tt.expected {
				t.Errorf("azureConnectionStringFromEnv() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestAzureInstrumentationKeyFromEnv tests the private azureInstrumentationKeyFromEnv helper
func TestAzureInstrumentationKeyFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "empty environment",
			envVars:  map[string]string{},
			expected: "",
		},
		{
			name: "uppercase env var set",
			envVars: map[string]string{
				"APPINSIGHTS_INSTRUMENTATIONKEY": "abc123-def456-ghi789",
			},
			expected: "abc123-def456-ghi789",
		},
		{
			name: "lowercase env var set",
			envVars: map[string]string{
				"appinsights_instrumentationkey": "xyz789-uvw456-rst123",
			},
			expected: "xyz789-uvw456-rst123",
		},
		{
			name: "uppercase takes precedence when both set",
			envVars: map[string]string{
				"APPINSIGHTS_INSTRUMENTATIONKEY": "uppercase-key",
			},
			expected: "uppercase-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			result := azureInstrumentationKeyFromEnv()
			if result != tt.expected {
				t.Errorf("azureInstrumentationKeyFromEnv() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestExtractInstrumentationKeyFromConnStr tests the private extractInstrumentationKeyFromConnStr helper
func TestExtractInstrumentationKeyFromConnStr(t *testing.T) {
	tests := []struct {
		name     string
		connStr  string
		expected string
	}{
		{
			name:     "empty string",
			connStr:  "",
			expected: "",
		},
		{
			name:     "missing key",
			connStr:  "IngestionEndpoint=https://example.com;LiveEndpoint=https://example.com",
			expected: "",
		},
		{
			name:     "full connection string with key first",
			connStr:  "InstrumentationKey=abc123;IngestionEndpoint=https://example.com",
			expected: "abc123",
		},
		{
			name:     "key appears later in string",
			connStr:  "IngestionEndpoint=https://example.com;InstrumentationKey=xyz789;LiveEndpoint=https://example.com",
			expected: "xyz789",
		},
		{
			name:     "connection string with extra spaces",
			connStr:  "IngestionEndpoint=https://example.com; InstrumentationKey = abc123 ;LiveEndpoint=https://example.com",
			expected: "abc123",
		},
		{
			name:     "mixed case key name lowercase",
			connStr:  "instrumentationkey=abc123;IngestionEndpoint=https://example.com",
			expected: "abc123",
		},
		{
			name:     "mixed case key name uppercase",
			connStr:  "INSTRUMENTATIONKEY=abc123;IngestionEndpoint=https://example.com",
			expected: "abc123",
		},
		{
			name:     "mixed case key name camelCase",
			connStr:  "instrumentationKey=abc123;IngestionEndpoint=https://example.com",
			expected: "abc123",
		},
		{
			name:     "key with no value",
			connStr:  "InstrumentationKey=;IngestionEndpoint=https://example.com",
			expected: "",
		},
		{
			name:     "key without equals",
			connStr:  "InstrumentationKey;IngestionEndpoint=https://example.com",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInstrumentationKeyFromConnStr(tt.connStr)
			if result != tt.expected {
				t.Errorf("extractInstrumentationKeyFromConnStr(%q) = %q, want %q", tt.connStr, result, tt.expected)
			}
		})
	}
}

// TestStructuredLogFields_AzureTraceparent tests the Azure log field enrichment in StructuredLogFields
// We need to test this carefully because AzureInstrumentationKey() uses sync.Once.
// We'll directly set the package-level variable to simulate the instrumentation key being configured.
func TestStructuredLogFields_AzureTraceparent(t *testing.T) {
	// Save original values
	origKey := azureInstrumentationKey
	origConnStr := azureConnectionString
	defer func() {
		azureInstrumentationKey = origKey
		azureConnectionString = origConnStr
	}()

	tests := []struct {
		name                    string
		traceparent             string
		instrumentationKey      string
		expectOperationID       bool
		expectOperationParentID bool
		expectedTraceID         string
		expectedParentIDPattern string
	}{
		{
			name:                    "no traceparent header",
			traceparent:             "",
			instrumentationKey:      "test-key",
			expectOperationID:       false,
			expectOperationParentID: false,
		},
		{
			name:                    "valid traceparent but no instrumentation key",
			traceparent:             "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			instrumentationKey:      "",
			expectOperationID:       false,
			expectOperationParentID: false,
		},
		{
			name:                    "valid traceparent with instrumentation key",
			traceparent:             "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			instrumentationKey:      "test-key-123",
			expectOperationID:       true,
			expectOperationParentID: false, // parseTraceParent doesn't extract span ID
			expectedTraceID:         "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			name:                    "valid traceparent with zero span ID",
			traceparent:             "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
			instrumentationKey:      "test-key-456",
			expectOperationID:       true,
			expectOperationParentID: false,
			expectedTraceID:         "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			name:                    "another valid traceparent",
			traceparent:             "00-12345678901234567890123456789012-abcdef1234567890-00",
			instrumentationKey:      "another-key",
			expectOperationID:       true,
			expectOperationParentID: false, // parseTraceParent doesn't extract span ID
			expectedTraceID:         "12345678901234567890123456789012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Directly set the package-level instrumentation key variable
			azureInstrumentationKey = tt.instrumentationKey

			// Create a fresh request with the traceparent header
			req := httptest.NewRequest("GET", "http://example.com", nil)
			if tt.traceparent != "" {
				req.Header.Set("traceparent", tt.traceparent)
			}

			// Call StructuredLogFields
			fields := StructuredLogFields(req)

			// Check operation_Id
			if tt.expectOperationID {
				if operationID, ok := fields["operation_Id"]; !ok {
					t.Errorf("expected operation_Id field to be set")
				} else if operationID != tt.expectedTraceID {
					t.Errorf("operation_Id = %q, want %q", operationID, tt.expectedTraceID)
				} else if len(operationID) != 32 {
					t.Errorf("operation_Id length = %d, want 32 (16-byte trace ID as hex)", len(operationID))
				}
			} else {
				if _, ok := fields["operation_Id"]; ok {
					t.Errorf("expected operation_Id field to NOT be set")
				}
			}

			// Check operation_ParentId
			if tt.expectOperationParentID {
				if operationParentID, ok := fields["operation_ParentId"]; !ok {
					t.Errorf("expected operation_ParentId field to be set")
				} else if operationParentID != tt.expectedParentIDPattern {
					t.Errorf("operation_ParentId = %q, want %q", operationParentID, tt.expectedParentIDPattern)
				}
			} else {
				if _, ok := fields["operation_ParentId"]; ok {
					t.Errorf("expected operation_ParentId field to NOT be set")
				}
			}

			// Ensure GCP fields are NOT set when no X-Cloud-Trace-Context header
			if _, ok := fields["logging.googleapis.com/trace"]; ok {
				t.Errorf("expected no GCP trace field when X-Cloud-Trace-Context header is not present")
			}
			if _, ok := fields["logging.googleapis.com/spanId"]; ok {
				t.Errorf("expected no GCP spanId field when X-Cloud-Trace-Context header is not present")
			}
		})
	}
}

// TestStructuredLogFields_NilRequest ensures StructuredLogFields handles nil request gracefully
func TestStructuredLogFields_NilRequest(t *testing.T) {
	fields := StructuredLogFields(nil)
	if fields != nil {
		t.Errorf("StructuredLogFields(nil) = %v, want nil", fields)
	}
}

// TestStructuredLogFields_AzureAndGCPIsolation ensures Azure and GCP fields don't interfere
func TestStructuredLogFields_AzureAndGCPIsolation(t *testing.T) {
	// Save original values
	origKey := azureInstrumentationKey
	defer func() {
		azureInstrumentationKey = origKey
	}()

	// Set instrumentation key for Azure
	azureInstrumentationKey = "azure-key"

	// Create request with only Azure traceparent header (no GCP header)
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	fields := StructuredLogFields(req)

	// Should have Azure fields
	if _, ok := fields["operation_Id"]; !ok {
		t.Errorf("expected operation_Id field when traceparent header is set")
	}

	// Should NOT have GCP fields
	if _, ok := fields["logging.googleapis.com/trace"]; ok {
		t.Errorf("expected no GCP trace field when only traceparent header is set")
	}
}
