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

### 2026-04-06 — Azure SDK Go Package Upgrade

**Task:** Upgrade all Azure SDK Go packages to latest stable without forcing AWS/GCP version changes.

**Findings:**

1. **azblob v0.6.1 → v1.6.4:** The source code in `runtimes/go/storage/objects/internal/providers/azblob/` was already written against the v1.x API (sub-packages `azblob/bloberror`, `azblob/container`, `azblob/sas`, `azblob/blob`, `azblob/blockblob`). The go.mod was simply never updated to match. No source changes needed.

2. **azservicebus v1.1.0 → v1.10.0:** Go module minor version bump. No API breakage. `go-amqp v1.4.0` pulled in as a new indirect dependency (replaces internal AMQP implementation).

3. **azidentity v1.10.1 → v1.13.1 and azcore v1.18.0 → v1.21.0:** Clean minor upgrades, no API changes affecting our code.

4. **azsecrets v1.4.0:** Already at latest stable — no change needed.

5. **AWS/GCP constraint upheld:** Zero AWS or GCP direct-dependency version changes. Shared transitive packages (`golang.org/x/crypto`, `x/net`, `x/sync`, `x/sys`, `x/text`) received minor/patch bumps pulled by Azure's newer deps — all acceptable.

6. **All tests pass:** azblob (bucket + uploader + SAS URL tests), azsecrets, pubsub/azure, cloudtrace, s3, pubsub/aws, metrics/aws, metrics/gcp — all green.

**Pattern for future Azure upgrades:** Always check if source code is already ahead of go.mod. The Azure SDK team ships Go sub-packages under the same module path across major versions (v0.x → v1.x same path), so the import paths don't change — only the go.mod needs updating.

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
