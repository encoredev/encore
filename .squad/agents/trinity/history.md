# Trinity — History

## Core Context

- **Project:** A versatile, polyglot squad for cloud-native projects spanning Azure, Kubernetes, Postgres, Redis, AWS, GCP, .NET, TypeScript, and Python.
- **Role:** Cloud/Infra
- **Joined:** 2026-04-06T20:34:16.104Z

## Learnings

<!-- Append learnings below -->

### 2026-04-06 — Azure Branch Cross-Cloud Safety Audit

**Task:** Audit `azure-support` branch for dependency issues, import cycles, and AWS/GCP behavioral regressions.

**Findings:**

1. **Shared files (logfields.go):** Azure block is fully additive. It gates on BOTH `traceparent` header AND `AzureInstrumentationKey() != ""` — independent of GCP's `X-Cloud-Trace-Context` check and AWS's `X-Amzn-Trace-Id` path. Zero risk of cross-cloud bleed.

2. **go mod verify:** Passed clean (`all modules verified`). Full `go build ./...` passes with no errors.

3. **Import graph:** All new Azure files import only Azure SDK packages, stdlib, and Encore-internal packages. No cross-cloud imports anywhere (verified by grep across all Azure files).

4. **Interface compliance:** `azureKVProvider` correctly implements `remoteSecretsProvider{FetchSecret}`. `azure.Exporter` correctly implements `metrics.exporter{Export, Shutdown}`. Config types implement `PubsubTopic`, `PubsubSubscription` interfaces via the same pattern as NSQ/SQS/GCP. All gated behind `//go:build !encore_no_azure`.

5. **Shared extractors:** `parseTraceParent` was already in the codebase before Azure additions. Azure only uses it in `logfields.go`, guarded by instrumentation key check. GCP, AWS, B3 parsers completely untouched.

6. **go.mod risk flags:**
   - `azblob v0.6.1` (pre-GA) with `azcore v1.18.0` (current) — old pre-GA SDK. Works due to module compatibility but should be upgraded to `azblob v1.x` before final merge.
   - `golang-jwt` jumped from v4 → v5 (breaking changes). Indirect dep only, no first-party code imports jwt directly. Low immediate risk.
   - `AzureAD/msal-go` v0.7.0 → v1.4.2 — major bump, indirect only.
   - `golang.org/x/crypto`, `net`, `sync`, `sys`, `text` all got significant version bumps.
   - `dnaeon/go-vcr` and `stretchr/testify` removed (were indirect, no first-party usage confirmed).

7. **All tests pass:** cloudtrace (23 tests), pubsub/azure (7 tests), config/infra, secrets, metadata, metrics (aws + gcp + azure + prometheus). No pre-existing failures caused by Azure changes.

**Verdict:** Safe to merge with one flag — `azblob v0.6.1` is outdated pre-GA SDK; recommend upgrade to `v1.x` before final merge for long-term supportability.

**Go vet pre-existing issues:** Unkeyed struct literal warnings in prometheus, gcp, aws test files — pre-existing, not introduced by Azure changes.

### 2025-01-XX — Azure Application Insights Cloud Trace Integration

**Files Created/Modified:**
- Created `runtimes/go/appruntime/shared/cloudtrace/azure.go` — Azure Application Insights resource discovery
- Modified `runtimes/go/appruntime/shared/cloudtrace/logfields.go` — Added Azure log correlation fields

**Key Implementation Details:**
- Azure Application Insights uses `operation_Id` (hex trace ID) and `operation_ParentId` (`|{traceId}.{spanId}.`) for log correlation
- Connection string discovery from env: `APPLICATIONINSIGHTS_CONNECTION_STRING` (preferred) or `APPINSIGHTS_INSTRUMENTATIONKEY` (legacy)
- Connection string format: `InstrumentationKey=<key>;IngestionEndpoint=https://...;...`
- Uses W3C `traceparent` header for trace context (vs GCP's `X-Cloud-Trace-Context`)
- Follows exact same pattern as GCP implementation: sync.Once for thread-safe lazy loading, recover() for panic safety, env var fallback chain with lowercase variants
