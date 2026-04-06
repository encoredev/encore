//go:build !encore_no_azure

package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// azureCollect looks up the azure collector from the registry and calls it.
// It also temporarily overrides azureIMDSEndpoint to use the provided URL.
func withIMDSEndpoint(t *testing.T, url string, fn func()) {
	t.Helper()
	orig := azureIMDSEndpoint
	azureIMDSEndpoint = url
	t.Cleanup(func() { azureIMDSEndpoint = orig })
	fn()
}

// collectAzure finds the "azure" collector in the registry and runs it.
func collectAzure(t *testing.T) (*ContainerMetadata, error) {
	t.Helper()
	for _, c := range collectorRegistry {
		if c.name == "azure" {
			return c.collect()
		}
	}
	t.Fatal("azure collector not found in registry")
	return nil, nil
}

// ---- successful IMDS response --------------------------------------------------------

func TestAzureCollector_SuccessfulResponse(t *testing.T) {
	const (
		vmID      = "aabbccdd-1234-5678-abcd-123456789012"
		rgName    = "my-resource-group"
		wantInstID = "56789012" // last 8 chars of vmID
	)

	body := buildIMDSResponse(vmID, rgName)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata") != "true" {
			http.Error(w, "missing Metadata header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	var got *ContainerMetadata
	var err error
	withIMDSEndpoint(t, srv.URL, func() {
		got, err = collectAzure(t)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ServiceID != rgName {
		t.Errorf("ServiceID: got %q, want %q", got.ServiceID, rgName)
	}
	if got.RevisionID != "" {
		t.Errorf("RevisionID: got %q, want empty", got.RevisionID)
	}
	if got.InstanceID != wantInstID {
		t.Errorf("InstanceID: got %q, want %q", got.InstanceID, wantInstID)
	}
}

// ---- field mapping tests -------------------------------------------------------------

func TestAzureCollector_FieldMapping(t *testing.T) {
	tests := []struct {
		name       string
		vmID       string
		rgName     string
		wantInstID string
	}{
		{
			name:       "long vmId – last 8 chars used",
			vmID:       "00000000-0000-0000-0000-000099887766",
			rgName:     "prod-rg",
			wantInstID: "99887766",
		},
		{
			name:       "short vmId – used as-is",
			vmID:       "short",
			rgName:     "dev-rg",
			wantInstID: "short",
		},
		{
			name:       "exactly 8 chars vmId",
			vmID:       "12345678",
			rgName:     "qa-rg",
			wantInstID: "12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := buildIMDSResponse(tt.vmID, tt.rgName)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			}))
			defer srv.Close()

			var got *ContainerMetadata
			var err error
			withIMDSEndpoint(t, srv.URL, func() {
				got, err = collectAzure(t)
			})

			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tt.name, err)
			}
			if got.ServiceID != tt.rgName {
				t.Errorf("%s: ServiceID: got %q, want %q", tt.name, got.ServiceID, tt.rgName)
			}
			if got.InstanceID != tt.wantInstID {
				t.Errorf("%s: InstanceID: got %q, want %q", tt.name, got.InstanceID, tt.wantInstID)
			}
		})
	}
}

// ---- unreachable IMDS ----------------------------------------------------------------

func TestAzureCollector_IMDSUnreachable(t *testing.T) {
	// Start and immediately close a server so the URL is valid but nothing listens.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := srv.URL
	srv.Close()

	var got *ContainerMetadata
	var err error
	withIMDSEndpoint(t, url, func() {
		got, err = collectAzure(t)
	})

	if err != nil {
		t.Fatalf("expected graceful empty return, got error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil ContainerMetadata, got nil")
	}
	if got.ServiceID != "" || got.InstanceID != "" {
		t.Errorf("expected empty metadata on IMDS failure, got %+v", got)
	}
}

// ---- helper --------------------------------------------------------------------------

// buildIMDSResponse constructs a minimal IMDS JSON response body.
func buildIMDSResponse(vmID, resourceGroupName string) []byte {
	payload := map[string]interface{}{
		"compute": map[string]interface{}{
			"location":          "eastus",
			"name":              "my-vm",
			"resourceGroupName": resourceGroupName,
			"subscriptionId":    "sub-12345",
			"vmId":              vmID,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("buildIMDSResponse: %v", err))
	}
	return b
}
