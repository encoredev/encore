# Implementation Plan: Encore CLI Customization (Strategy 3)

## Scope and Non-Goals
**Scope**:
- Configuration of Web Dashboard URL (`ENCORE_WEBDASH_URL`) via safe getter functions.
- Replacement of user-facing dashboard URLs in CLI output with normalized paths.
- Optional configuration of Documentation URL (`ENCORE_DOCS_URL`).
- Optional Authentication customization (API Key support) reusing existing platform contracts.

**Non-Goals**:
- Rewriting the platform client architecture.
- Full whitelabeling of all CLI help text.
- Changing the `encore.app` metadata format.

## Repository Touch Points
All paths relative to `vendor/encore/` (User must confirm if operating on fork or vendor dir).

1.  **Utilities (New Environment)**:
    -   `internal/urlutil/join.go`: Create new package to hold `JoinURL` to avoid cycles in `conf`.

2.  **Configuration**:
    -   `internal/conf/conf.go`: Add `WebDashBaseURL()` and `DocsBaseURL()`.

3.  **CLI Command Output**:
    -   `cli/cmd/encore/deploy.go`: Update "Started Deploy" URL.
    -   `cli/cmd/encore/app/create.go`: Update "Web URL".
    -   `cli/cmd/encore/app/initialize.go`: Update "Cloud Dashboard" URL.

4.  **Documentation URLs**:
    -   Various call sites (e.g. `k8s/config.go`, `telemetry.go`) replacing hardcoded strings.

5.  **Authentication**:
    -   `cli/cmd/encore/auth/auth.go`: Refactor `authCmd` to package-level variable (unexported) so it can be accessed by sibling files.
    -   `cli/cmd/encore/auth/apikey.go`: Add `login-apikey` subcommand which reuses `encr.dev/cli/internal/login` logic.

## Plan of Record (Phased Steps)

### Phase 0: Inventory & Safe Prep
-   **Check Repo State**: Confirm `vendor/encore` is writable.
-   **Inventory**: Verify `https://encore.dev` occurrences.
-   **Structure Check**: Verify `cli/cmd/encore/auth` package structure to ensure clean integration of new command.

### Phase 1: Config & Utils (Testable)
Implement configuration and utility functions.

**Actions**:
-   **`internal/urlutil/join.go`**:
    -   Implement `JoinURL(base, relPath string) string`.
    -   Guard: Check if `relPath` is absolute URL.
    -   Edge Case: If `base` is empty, return normalized `relPath` (no leading slash).
-   **`internal/conf/conf.go`**:
    -   Define constants: `defaultWebDashURL`, `defaultDocsURL`.
    -   Implement `WebDashBaseURL() string`: returns env override or default, trimmed of trailing slash.
    -   Implement `DocsBaseURL() string`: returns env override or default, trimmed of trailing slash.

### Phase 2: Replace Dashboard Output URLs
Wire up the new config to the CLI commands using `JoinURL`.

**Actions**:
-   **Deploy (`cli/cmd/encore/deploy.go`)**:
    -   Construct path: `rel := fmt.Sprintf("/%s/deploys/%s/%s", appSlug, rollout.EnvName, strings.TrimPrefix(rollout.ID, "roll_"))`
    -   URL: `url := urlutil.JoinURL(conf.WebDashBaseURL(), rel)`
-   **App Create** & **Init**: Similar pattern with `/app-slug`.

### Phase 3: Optional Docs URL
Customizable documentation links with path preservation.

**Mapping Table**:
| Found Literal | Replacement Logic |
| :--- | :--- |
| `https://encore.dev` | `conf.DocsBaseURL()` |
| `https://encore.dev/docs/deploying/kubernetes` | `urlutil.JoinURL(conf.DocsBaseURL(), "/docs/deploying/kubernetes")` |
| `https://encore.dev/*` | `urlutil.JoinURL(conf.DocsBaseURL(), "/original/path")` |

**Verification**: Grep for `https://encore.dev` after changes.

### Phase 4: Optional Auth Customization
Add `login-apikey` command.

**Actions**:
-   **Modify `cli/cmd/encore/auth/auth.go`**:
    -   Lift `authCmd` from `init()` scope to package-level `var authCmd = ...` (unexported).
    -   Ensure `root.Cmd.AddCommand(authCmd)` remains in `init()` or runs correctly.
-   **Create `cli/cmd/encore/auth/apikey.go`**:
    -   Define `loginApiKeyCmd` (Use: `login-apikey`).
    -   Replicate logic from `DoLoginWithAuthKey`: call `login.WithAuthKey(key)` then `conf.Write(cfg)`.
    -   Register with `authCmd.AddCommand(loginApiKeyCmd)` in `init()`.

### Phase 5: Tests & Smoke
-   **Unit Tests (`internal/urlutil/join_test.go`)**: Test `JoinURL` edge cases.
-   **Unit Tests (`internal/conf/conf_test.go`)**: Test `WebDashBaseURL` overrides.
-   **Smoke Tests**: Build binary, run with env vars, verify output URLs.

## Concrete Patches

### internal/urlutil/join.go

```go
package urlutil

import "strings"

// JoinURL joins a base URL and a relative path, ensuring exactly one slash.
// It guards against accidental full URLs in relPath.
func JoinURL(base, relPath string) string {
    // Guard: If relPath is actually a full URL, return it as-is to prevent mangling.
    if strings.HasPrefix(relPath, "http://") || strings.HasPrefix(relPath, "https://") {
        return relPath
    }
    // If base is empty, return cleaned relative path to avoid leading slash being interpreted as root
    if strings.TrimSpace(base) == "" {
        return strings.TrimLeft(relPath, "/")
    }
    return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(relPath, "/")
}
```

### internal/conf/conf.go

```go
const (
    defaultWebDashURL = "https://app.encore.cloud"
    defaultDocsURL    = "https://encore.dev"
)

// WebDashBaseURL returns the base URL for the Web Dashboard.
func WebDashBaseURL() string {
    u := os.Getenv("ENCORE_WEBDASH_URL")
    if u == "" {
        u = defaultWebDashURL
    }
    return strings.TrimRight(u, "/")
}

// DocsBaseURL returns the base URL for documentation.
func DocsBaseURL() string {
    u := os.Getenv("ENCORE_DOCS_URL")
    if u == "" {
        u = defaultDocsURL
    }
    return strings.TrimRight(u, "/")
}
```

## Testing Strategy

### Unit Tests

`internal/urlutil/join_test.go`:
```go
func TestJoinURL(t *testing.T) {
    cases := []struct{ base, path, want string }{
        {"https://a.com", "x", "https://a.com/x"},
        {"https://a.com/", "/x", "https://a.com/x"},
        {"https://a.com", "https://b.com", "https://b.com"}, // Guard check
        {"", "/x", "x"}, // Empty base check
    }
    // ... loop run ...
}
```

## Acceptance Criteria
- [ ] **Exact URL Output**: `encore deploy` prints a URL starting with `ENCORE_WEBDASH_URL` (if set) + exact relative path, with no double slashes.
- [ ] **Docs Replaced**: All `https://encore.dev/*` links usage reflects `ENCORE_DOCS_URL` (if set).
- [ ] **JoinURL Guard**: If `urlutil.JoinURL` is called with a full URL as `relPath`, it returns the full URL unchanged.
- [ ] **JoinURL Empty Base**: If `base` is empty, returns path without leading slash.
- [ ] **Auth Compatibility**: After `encore auth login-apikey`, an authenticated command (e.g. `encore app list`) succeeds without further configuration.
