# Encore CLI Customization Worklog & Maintenance Plan

## 1. Worklog: Customization for Self-Hosted Backend & ACL

This log details the specific code modifications applied to `vendor/encore` to support a self-hosted platform, custom documentation, and API Key authentication ("ACL").

### 1.1. Core Configuration & Utilities
*   **`internal/conf/conf.go`**:
    *   Added support for `ENCORE_WEBDASH_URL` (default: `https://app.encore.cloud`).
    *   Added support for `ENCORE_DOCS_URL` (default: `https://encore.dev`).
    *   Implemented `WebDashBaseURL()` and `DocsBaseURL()` getters with environment variable overrides.
*   **`internal/urlutil/join.go`** (New File):
    *   Implemented `JoinURL(base, rel)` to safely concatenate base URLs with paths, handling potential double slashes or missing bases.
*   **`internal/urlutil/join_test.go`** (New File):
    *   Unit tests for `JoinURL` coverage.

### 1.2. Authentication & ACL
*   **`cli/cmd/encore/auth/apikey.go`** (New File):
    *   Implemented `login-apikey` subcommand.
    *   Reads API key from `--auth-key` flag or `stdin`.
    *   Uses standard Encore config storage (`conf.Write`), ensuring compatibility with the daemon and other tools.
    *   **Purpose**: Enables CI/CD systems and Service Accounts (ACL) to authenticate without interactive browser sessions.
*   **`cli/cmd/encore/auth/auth.go`**:
    *   Refactored `authCmd` to package-level scope to allow `apikey.go` to register the subcommand.

### 1.3. Documentation & Algolia Search
*   **`cli/daemon/mcp/docs_tools.go`**:
    *   Replaced hardcoded Algolia Application ID and API Key with environment variables:
        *   `ENCORE_DOCS_SEARCH_APP_ID`
        *   `ENCORE_DOCS_SEARCH_API_KEY`
    *   Updated `getDocs` to use `conf.DocsBaseURL()` instead of hardcoded `https://encore.dev`.
    *   Refactored URL construction to use `urlutil.JoinURL`.

### 1.4. CLI Command URL Replacements
The following files were modified to use `WebDashBaseURL()` or `DocsBaseURL()` instead of hardcoded strings:

*   **`cli/cmd/encore/deploy.go`**: Deploy status URL.
*   **`cli/cmd/encore/app/create.go`**: App creation success URL.
*   **`cli/cmd/encore/app/initialize.go`**: Init success URL.
*   **`cli/cmd/encore/version.go`**: "Install Encore" link in update check.
*   **`cli/cmd/encore/k8s/config.go`**: Install hint URL.
*   **`cli/cmd/encore/telemetry.go`**: Telemetry docs link.
*   **`cli/daemon/export/infra_config.go`**: Self-hosting docs link.
*   **`cli/daemon/run/proc_groups.go`**: Secrets documentation link.

---

## 2. Maintenance Plan: Minimizing the Patch Cycle

To keep the `aurum` monorepo healthy while maintaining these customizations ("patches"), we propose the following strategy.

### 2.1. Upstream Candidates (High Priority)
The most effective way to eliminate patch maintenance is to merge these changes into the upstream `encoredev/encore` repository.

1.  **Configuration Overrides**: Submit a PR to support `ENCORE_WEBDASH_URL` and `ENCORE_DOCS_URL`. This is a low-risk, high-value change for *any* Encore user needing custom pointers.
2.  **API Key Login**: Submit `login-apikey` as a feature request/PR. It provides standard non-interactive login support which is broadly useful.
3.  **Docs Search**: Submit the Algolia env-var support as a PR.

**Benefit**: Once accepted, these files return to being standard upstream code, removing merge conflicts.

### 2.2. Conflict Resolution Strategy (Until Upstreamed)
Since `vendor/encore` is a `git subtree`, updates via `scripts/sync-upstream-encore.sh` will trigger merge conflicts if upstream modifies lines we touched.

*   **Protected Files**: `internal/conf/conf.go`, `cli/cmd/encore/auth/auth.go`, `cli/daemon/mcp/docs_tools.go`.
*   **Strategy**:
    1.  **Accept Upstream for Logic**: If significant logic changes occur in `auth.go` or `conf.go`, accept upstream changes first.
    2.  **Re-apply Patches**: Re-introduce the specific env var getters and subcommand registration.
    3.  **New Files**: `apikey.go` and `urlutil/` packages are safe; they will likely never conflict unless upstream adds identically named files (unlikely).

### 2.3. Automated Verification
To prevent silent regressions (e.g., a merge overwriting our URL getters without a conflict), we must run verification tests post-sync.

**Action**: Add a `verify-custom-cli` step to the CI pipeline or `sync` script.
```bash
# Verification Command
go test ./internal/conf/... ./cli/cmd/encore/auth/...
# Smoke Check
./encore auth login-apikey --help
```

### 2.4. Documentation of Deviation
Maintain this `worklog` as the source of truth for "what is different". When resolving conflicts, refer to **Section 1.4** to ensure all URL replacements are re-applied.
