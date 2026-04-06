# Squad Decisions

## Active Decisions

### Azure Test Coverage Implementation — 2026-04-06

**Decision:** Complete Azure test coverage as requested by coverage audit.

**Status:** Implemented ✅

**Key Outcomes:**
- 8 tests for Azure Key Vault secrets provider (httptest TLS mock pattern)
- 9 tests for Azure config validation (table-driven approach)
- Extended infra.config.azure.json with test data
- All 17 tests passing

**Rationale:** Production-quality coverage required before merging azure-support branch. httptest.NewTLSServer + fake credential pattern provides reliable testing without real cloud resources.

## Archived Inbox Items

### 2026-04-06: Azure Test Coverage Audit Findings
**By:** Ryan Graham (via Squad)

Azure support test coverage audit identified:
- CRITICAL: azure_keyvault.go has ZERO tests — FetchSecret error paths, nil response handling, credential failures all untested
- HIGH: AzureMonitor.Validate() in infra/config.go has no error-path tests
- HIGH: AzureServiceBusPubsub.DeleteTopic() and AzureTopic.DeleteSubscription() methods untested
- HIGH: azure_monitor_exporter.go metadata collection failure path untested
- MEDIUM: Azure Monitor config missing from infra.config.azure.json test data
- Already well-tested: azure_collector.go, azure_monitor.go, azblob bucket, config parsing
- Rust tests blocked by pre-existing vcruntime.h build env issue (not Azure code bug)

### Azure Cloud Trace Integration — 2026-04-06

**Decision:** Azure Application Insights cloud trace integration added following GCP Cloud Trace pattern.

**Implementation:**
- Log correlation fields: `operation_Id` (hex-encoded trace ID), `operation_ParentId` (Application Insights format)
- Resource discovery from env vars: `APPLICATIONINSIGHTS_CONNECTION_STRING` (preferred) or `APPINSIGHTS_INSTRUMENTATIONKEY` (fallback)
- Uses W3C `traceparent` header (OpenTelemetry standard)

**Files:**
- Created: `runtimes/go/appruntime/shared/cloudtrace/azure.go`
- Modified: `runtimes/go/appruntime/shared/cloudtrace/logfields.go`

**Rationale:** Parity with GCP pattern. No Azure IMDS querying needed. Connection string preferred per modern Azure SDKs.

**Status:** ✅ Implemented. Build and vet pass.

### Azure Cloud Trace Tests — White-Box Testing Pattern — 2026-04-06

**Decision:** Use white-box testing pattern for Azure cloudtrace tests due to `sync.Once` isolation challenges.

**Challenge:** `sync.Once` fires once per process lifetime; env var changes via `t.Setenv()` have no effect after firing, breaking traditional black-box testing across subtests.

**Solution:**
1. Test file declared as `package cloudtrace` (not `cloudtrace_test`)
2. Test private helpers directly: `azureConnectionStringFromEnv()`, `azureInstrumentationKeyFromEnv()`, `extractInstrumentationKeyFromConnStr()`
3. For integration tests, directly manipulate package variables (`azureInstrumentationKey`) with defer cleanup

**Benefits:** Test isolation, determinism (no execution-order deps), clarity (helper vs integration), full coverage of unit and integration flows.

**Test Coverage:** 23 tests covering env var resolution, connection string parsing (10 edge cases), log field enrichment with traceparent, nil requests, Azure/GCP field isolation.

**Status:** ✅ Implemented. All 23 tests passing with 100% coverage.

**Pattern Reference:** For future `sync.Once` testing: white-box (`package X`), test helpers directly, manipulate state with cleanup, document in comments.

## Governance

- All meaningful changes require team consensus
- Document architectural decisions here
- Keep history focused on work, decisions focused on direction
