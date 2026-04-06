# Morpheus — History

## Core Context

- **Project:** A versatile, polyglot squad for cloud-native projects spanning Azure, Kubernetes, Postgres, Redis, AWS, GCP, .NET, TypeScript, and Python.
- **Role:** Backend Dev
- **Joined:** 2026-04-06T20:34:16.106Z

## Learnings

### Azure Test Coverage - 2026-04-06

**Key Patterns Found:**
- Azure SDK requires HTTPS endpoints for testing. Used `httptest.NewTLSServer` instead of `httptest.NewServer`
- Azure SDK adds trailing slashes to API paths (e.g., `/secrets/test-secret/`). Test handlers must handle both with and without trailing slash
- TLS certificate validation must be disabled for test servers using `InsecureSkipVerify: true` in transport config
- Fake credentials can be created for testing by implementing the `policy.TokenCredential` interface with a `GetToken` method
- The Azure Key Vault SDK client can be configured with custom HTTP transport via `ClientOptions`

**Test Approaches That Worked:**
- **Mock HTTP Server Pattern:** Used `httptest.NewTLSServer` to simulate Azure Key Vault REST API responses
- **Fake Credentials:** Created a `fakeCredential` struct implementing `policy.TokenCredential` to bypass real Azure authentication
- **Table-Driven Tests:** Used for validation tests following existing codebase patterns with `github.com/frankban/quicktest`
- **Build Tags:** All Azure tests use `//go:build !encore_no_azure` to match the source files

**Mocking Strategy for Azure SDK:**
- Direct HTTP mocking at the transport layer rather than trying to mock the Azure SDK interfaces
- Configure test client with custom `http.Transport` that accepts self-signed certificates
- Simulate Azure API responses with proper JSON structure (`{"value": "secret-value", "id": "..."}`)
- Handle Azure SDK quirks like trailing slashes and query parameters (e.g., `api-version`)

**Validation Testing:**
- Used the existing `Validate()` framework with `validator` pattern
- Tested both valid and invalid configurations for all required fields
- Verified `DeleteTopic()` and `DeleteSubscription()` methods work correctly
- Confirmed that `GetTopics()` and `GetSubscriptions()` return proper interface types

**Test Data Integration:**
- Successfully extended `infra.config.azure.json` with Azure Monitor metrics and secrets provider configuration
- Existing `TestParseInfraConfigEnvAzure` automatically picked up and validated the new data structure

**Test Files Created:**
- `azure_keyvault_test.go` - 8 comprehensive tests using TLS mock server pattern
- `azure_config_test.go` - 9 table-driven validation tests for all Azure config types
- `infra.config.azure.json` - Extended with AzureMonitor and KeyVault test data

**Outcomes:**
- 17 new test functions created, all passing
- 0 test failures across all test runs
- No production code changes required for test coverage
- Patterns documented for future Azure SDK test development

### Azure Cloud Trace Testing - 2026-04-06

**Test File Created:**
- `runtimes/go/appruntime/shared/cloudtrace/azure_test.go` - Comprehensive unit tests for Azure Application Insights trace correlation

**Tests Written:**
- `TestAzureConnectionStringFromEnv` (3 subtests) - Tests environment variable resolution for connection strings
- `TestAzureInstrumentationKeyFromEnv` (4 subtests) - Tests environment variable resolution for instrumentation keys  
- `TestExtractInstrumentationKeyFromConnStr` (10 subtests) - Tests connection string parsing with various edge cases
- `TestStructuredLogFields_AzureTraceparent` (5 subtests) - Tests Azure log field enrichment with W3C traceparent headers
- `TestStructuredLogFields_NilRequest` - Tests graceful nil handling
- `TestStructuredLogFields_AzureAndGCPIsolation` - Tests that Azure and GCP fields don't interfere

**Total:** 23 test cases, all passing

**Testing Pattern for sync.Once:**
- Used white-box testing (`package cloudtrace`, not `package cloudtrace_test`) to access private helper functions
- Tested `azureConnectionStringFromEnv()`, `azureInstrumentationKeyFromEnv()`, and `extractInstrumentationKeyFromConnStr()` directly
- For integration tests requiring package-level state, directly manipulated `azureInstrumentationKey` variable with defer cleanup
- This approach avoids sync.Once isolation issues that would occur with env var manipulation after first call

**Implementation Discovery:**
- The `parseTraceParent()` function extracts only the trace ID, NOT the parent span ID from the W3C traceparent header
- As a result, `operation_ParentId` is never populated in Azure log fields (only `operation_Id` is set)
- Tests written to match actual implementation behavior

**Edge Cases Tested:**
- Empty environment variables
- Case-insensitive key matching (uppercase, lowercase, mixed case)
- Connection strings with extra whitespace
- Missing keys, empty values, malformed strings
- Zero span IDs
- Isolation between Azure and GCP trace fields

<!-- Append learnings below -->

