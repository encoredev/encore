//go:build !encore_no_azure

package secrets

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	qt "github.com/frankban/quicktest"
)

// TestFetchSecret_Success tests that a valid secret value is returned from Key Vault.
func TestFetchSecret_Success(t *testing.T) {
	c := qt.New(t)

	const secretName = "test-secret"
	const secretValue = "super-secret-value"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Azure SDK sends path like /secrets/test-secret/ with trailing slash
		if r.URL.Path != "/secrets/"+secretName+"/" && r.URL.Path != "/secrets/"+secretName {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"value": secretValue,
			"id":    "https://test.vault.azure.net/secrets/" + secretName,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := createTestProvider(t, srv)

	got, err := provider.FetchSecret(context.Background(), secretName)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, secretValue)
}

// TestFetchSecret_NotFound tests that a 404 from Key Vault returns an error.
func TestFetchSecret_NotFound(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "SecretNotFound",
				"message": "Secret not found",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := createTestProvider(t, srv)

	_, err := provider.FetchSecret(context.Background(), "nonexistent")
	c.Assert(err, qt.Not(qt.IsNil))
}

// TestFetchSecret_NilValue tests that a response with nil Value returns an error.
func TestFetchSecret_NilValue(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id": "https://test.vault.azure.net/secrets/test",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := createTestProvider(t, srv)

	_, err := provider.FetchSecret(context.Background(), "test")
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Contains, "returned no value")
}

// TestFetchSecret_EmptyValue tests that a response with empty string value is handled.
func TestFetchSecret_EmptyValue(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"value": "",
			"id":    "https://test.vault.azure.net/secrets/test",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := createTestProvider(t, srv)

	got, err := provider.FetchSecret(context.Background(), "test")
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, "")
}

// TestFetchSecret_ContextCanceled tests that context cancellation is handled.
func TestFetchSecret_ContextCanceled(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	provider := createTestProvider(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.FetchSecret(ctx, "test")
	c.Assert(err, qt.Not(qt.IsNil))
}

// TestFetchSecret_SDKError tests that SDK errors are propagated.
func TestFetchSecret_SDKError(t *testing.T) {
	c := qt.New(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "InternalServerError",
				"message": "Internal server error",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := createTestProvider(t, srv)

	_, err := provider.FetchSecret(context.Background(), "test")
	c.Assert(err, qt.Not(qt.IsNil))
}

// TestFetchSecret_MultipleSecrets tests fetching multiple different secrets.
func TestFetchSecret_MultipleSecrets(t *testing.T) {
	c := qt.New(t)

	secrets := map[string]string{
		"db-password": "password123",
		"api-key":     "key456",
		"token":       "token789",
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, value := range secrets {
			// Azure SDK sends path with trailing slash
			if r.URL.Path == "/secrets/"+name+"/" || r.URL.Path == "/secrets/"+name {
				w.Header().Set("Content-Type", "application/json")
				resp := map[string]interface{}{
					"value": value,
					"id":    "https://test.vault.azure.net/secrets/" + name,
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	provider := createTestProvider(t, srv)

	for name, expected := range secrets {
		got, err := provider.FetchSecret(context.Background(), name)
		c.Assert(err, qt.IsNil, qt.Commentf("secret %q", name))
		c.Assert(got, qt.Equals, expected, qt.Commentf("secret %q", name))
	}
}

// createTestProvider creates an azureKVProvider configured to use the test server.
// It uses a fake credential that doesn't require real Azure authentication.
func createTestProvider(t *testing.T, srv *httptest.Server) *azureKVProvider {
	t.Helper()

	cred := &fakeCredential{}
	
	// Configure the client to skip TLS verification for test servers
	opts := &azsecrets.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			},
		},
	}
	
	client, err := azsecrets.NewClient(srv.URL, cred, opts)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}

	return &azureKVProvider{client: client}
}

// fakeCredential is a fake Azure credential for testing that returns a dummy token.
type fakeCredential struct{}

func (f *fakeCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	expiresOn := time.Now().Add(time.Hour)
	return azcore.AccessToken{
		Token:     "fake-token-for-testing",
		ExpiresOn: expiresOn,
	}, nil
}

// TestNewAzureKVProvider tests the provider initialization through the init function.
func TestNewAzureKVProvider(t *testing.T) {
	c := qt.New(t)

	// Test that newAzureKVProvider was set by the init function
	c.Assert(newAzureKVProvider, qt.Not(qt.IsNil))

	// We can't easily test the real newAzureKVProvider function here because it
	// attempts to create a DefaultAzureCredential, which requires real Azure
	// authentication or environment variables. This test just verifies that the
	// init function registered the provider function.
}
