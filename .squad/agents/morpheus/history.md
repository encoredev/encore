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

<!-- Append learnings below -->

