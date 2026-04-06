# Trinity — History

## Core Context

- **Project:** A versatile, polyglot squad for cloud-native projects spanning Azure, Kubernetes, Postgres, Redis, AWS, GCP, .NET, TypeScript, and Python.
- **Role:** Cloud/Infra
- **Joined:** 2026-04-06T20:34:16.104Z

## Learnings

<!-- Append learnings below -->

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
