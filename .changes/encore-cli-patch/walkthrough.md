# Encore CLI Customization Walkthrough

This document outlines the modifications made to the Encore CLI to support a self-hosted backend.

## Feature Overview

We successfully implemented "Strategy 3: Code Modifications", enabling dynamic configuration of the Encore CLI via environment variables. This allows the CLI to point to a self-hosted Platform API, Web Dashboard, and Documentation.

### 1. Dynamic URL Configuration
We replaced hardcoded URLs (e.g., `https://app.encore.cloud`, `https://encore.dev`) with dynamic getters.

*   **Configurable Environment Variables:**
    *   `ENCORE_WEBDASH_URL`: Base URL for the Web Dashboard. Defaults to `https://app.encore.cloud`.
    *   `ENCORE_DOCS_URL`: Base URL for Documentation. Defaults to `https://encore.dev`.

*   **Implementation:**
    *   Added getters in `internal/conf/conf.go`.
    *   Created `internal/urlutil/join.go` for safe URL concatenation.
    *   Updated CLI commands (`deploy`, `app create`, `app init`, etc.) to use these getters.

### 2. Custom Authentication (`login-apikey`)
We added a new authentication method to support API Key login, essential for self-hosted environments or CI/CD integration where interactive browser login is not feasible.

*   **New Command:** `encore auth login-apikey --auth-key=<KEY>`
*   **Behavior:** Seamlessly integrates with the existing authentication system, storing credentials securely in the standard Encore config location.

## Changes Verified

### Build Verification
The CLI was successfully rebuilt with all changes:
```bash
go build -o encore-custom ./cli/cmd/encore
```

### Smoke Tests
1.  **Version Command:** Verified `encore version update` uses the configurable documentation URL in error messages.
2.  **Auth Command:** Verified `encore auth login-apikey` is available and help text is correct.
3.  **Docs Links:** Verified links in `telemetry`, `infra_config`, etc., use the configurable base URL.

## Usage Guide for Self-Hosted Backend

To use the custom CLI with your self-hosted backend, set the following environment variables:

```bash
export ENCORE_PLATFORM_API_URL="https://api.your-encore-instance.com"
export ENCORE_WEBDASH_URL="https://dashboard.your-encore-instance.com"
export ENCORE_DOCS_URL="https://docs.your-encore-instance.com"
```

Then run the CLI as usual:
```bash
./encore-custom app create my-app
```

## Next Steps
*   Distribute the `encore-custom` binary to your team.
*   Update your CI/CD pipelines to set the necessary environment variables.
