package cloudtrace

import (
	"os"
	"strings"
	"sync"
)

var (
	azureInstrumentationKey string
	azureConnectionString   string
	azureResourceLoad       sync.Once
)

// AzureInstrumentationKey returns the Azure Application Insights instrumentation key.
// Returns empty string if not configured.
func AzureInstrumentationKey() string {
	azureResourceLoad.Do(loadAzureResourceInfo)
	return azureInstrumentationKey
}

// AzureConnectionString returns the Azure Application Insights connection string.
// Returns empty string if not configured.
func AzureConnectionString() string {
	azureResourceLoad.Do(loadAzureResourceInfo)
	return azureConnectionString
}

func loadAzureResourceInfo() {
	// recover from any panics
	defer func() {
		if r := recover(); r != nil {
			azureConnectionString = ""
			azureInstrumentationKey = ""
		}
	}()

	// Check connection string first (preferred over standalone instrumentation key)
	connStr := azureConnectionStringFromEnv()
	if connStr != "" {
		azureConnectionString = connStr
		azureInstrumentationKey = extractInstrumentationKeyFromConnStr(connStr)
		return
	}

	// Fall back to standalone instrumentation key
	azureInstrumentationKey = azureInstrumentationKeyFromEnv()
}

func azureConnectionStringFromEnv() string {
	for _, key := range []string{
		"APPLICATIONINSIGHTS_CONNECTION_STRING",
		"applicationinsights_connection_string",
	} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func azureInstrumentationKeyFromEnv() string {
	for _, key := range []string{
		"APPINSIGHTS_INSTRUMENTATIONKEY",
		"appinsights_instrumentationkey",
	} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

// extractInstrumentationKeyFromConnStr parses the InstrumentationKey from an
// Application Insights connection string.
// Format: "InstrumentationKey=<key>;IngestionEndpoint=https://...;..."
func extractInstrumentationKeyFromConnStr(connStr string) string {
	for _, part := range strings.Split(connStr, ";") {
		if strings.EqualFold(strings.TrimSpace(strings.SplitN(part, "=", 2)[0]), "InstrumentationKey") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				return strings.TrimSpace(kv[1])
			}
		}
	}
	return ""
}
