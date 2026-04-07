# Trinity: Azure SDK Upgrade — 2026-04-06T235546Z

**Agent:** Trinity  
**Task:** Azure SDK package upgrades  
**Commit:** 458dc912  

## Work Performed

Upgraded all Azure SDK Go packages in `runtimes/go/` to latest stable versions:
- `azblob` v0.6.1 → v1.6.4 (pre-GA → stable)
- `azcore` v1.18.0 → v1.21.0
- `azidentity` v1.10.1 → v1.13.1
- `azservicebus` v1.1.0 → v1.10.0
- `azsecrets` v1.4.0 (no change)

AWS and GCP dependencies remain frozen.

## Verification

- `go build ./...` — ✅
- Azure pubsub tests — ✅
- Azure secrets tests — ✅
- Azure storage tests — ✅
- AWS/GCP tests — ✅

## Outcome

✅ Merged to main. Reduced security surface. All tests passing.
