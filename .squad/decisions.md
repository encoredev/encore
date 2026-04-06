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

## Governance

- All meaningful changes require team consensus
- Document architectural decisions here
- Keep history focused on work, decisions focused on direction
