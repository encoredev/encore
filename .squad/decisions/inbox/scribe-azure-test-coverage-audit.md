### 2026-04-06: Azure Test Coverage Audit Findings
**By:** Ryan Graham (via Squad)
**What:** Azure support test coverage audit identified:
- CRITICAL: azure_keyvault.go has ZERO tests — FetchSecret error paths, nil response handling, credential failures all untested
- HIGH: AzureMonitor.Validate() in infra/config.go has no error-path tests
- HIGH: AzureServiceBusPubsub.DeleteTopic() and AzureTopic.DeleteSubscription() methods untested
- HIGH: azure_monitor_exporter.go metadata collection failure path untested
- MEDIUM: Azure Monitor config missing from infra.config.azure.json test data
- Already well-tested: azure_collector.go, azure_monitor.go, azblob bucket, config parsing
- Rust tests blocked by pre-existing vcruntime.h build env issue (not Azure code bug)
**Why:** Ensure production-quality coverage before merging azure-support branch
