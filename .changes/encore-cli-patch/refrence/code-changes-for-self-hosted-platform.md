# Encore CLI: Code Changes for Self-Hosted Platform

**Author:** Manus AI  
**Date:** January 29, 2026

This document provides a detailed guide for modifying the Encore CLI to redirect all cloud API calls to a custom, self-hosted platform. It includes specific file changes, code examples, and implementation strategies.

## Executive Summary

**Good News:** The Encore CLI is already designed to support custom platform URLs through environment variables. **Minimal code changes are required** for basic redirection.

**Required Changes:**
1. **Zero changes** for basic API redirection (use `ENCORE_PLATFORM_API_URL`)
2. **~15 lines** to add web dashboard URL configuration
3. **~50 lines** to customize hardcoded UI URLs (optional)
4. **~100 lines** to implement custom authentication (optional)

## Configuration Architecture

### Current Design

The Encore CLI uses a **three-tier configuration system**:

```
┌─────────────────────────────────────────────┐
│  1. Environment Variables (Highest Priority)│
│     ENCORE_PLATFORM_API_URL                 │
│     ENCORE_DEVDASH_URL                      │
└─────────────────┬───────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│  2. Build-time Linker Flags                 │
│     -X encr.dev/internal/conf.              │
│        defaultPlatformURL=...               │
└─────────────────┬───────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│  3. Hardcoded Defaults (Lowest Priority)    │
│     https://api.encore.cloud                │
│     https://devdash.encore.dev              │
└─────────────────────────────────────────────┘
```

### Key Configuration Points

**File:** `internal/conf/conf.go`

```go
// Lines 28-32: Default URLs
var (
    defaultPlatformURL     = "https://api.encore.cloud"
    defaultDevDashURL      = "https://devdash.encore.dev"
    defaultConfigDirectory = "encore"
)

// Lines 34-40: API Base URL (Environment Variable Override)
var APIBaseURL = (func() string {
    if u := os.Getenv("ENCORE_PLATFORM_API_URL"); u != "" {
        return u
    }
    return defaultPlatformURL
})()

// Lines 42-45: WebSocket URL (Auto-derived)
var WSBaseURL = (func() string {
    return strings.Replace(APIBaseURL, "http", "ws", -1)
})()

// Lines 47-53: Dev Dashboard URL (Environment Variable Override)
var DevDashURL = (func() string {
    if u := os.Getenv("ENCORE_DEVDASH_URL"); u != "" {
        return u
    }
    return defaultDevDashURL
})()
```

## Modification Strategy

### Strategy 1: Environment Variables Only (Recommended)

**Effort:** Zero code changes  
**Use Case:** Testing, development, temporary redirection

**Implementation:**

```bash
# Set your custom platform URL
export ENCORE_PLATFORM_API_URL="https://api.mycompany.com"
export ENCORE_DEVDASH_URL="https://devdash.mycompany.com"

# Run Encore CLI
encore run
```

**Pros:**
- No code changes required
- Easy to switch between platforms
- Works with official Encore binaries

**Cons:**
- Must set environment variables every time
- Doesn't change hardcoded UI URLs (deploy success messages, etc.)

---

### Strategy 2: Build-Time Linker Flags

**Effort:** Zero code changes, custom build command  
**Use Case:** Distribution of custom CLI builds

**Implementation:**

```bash
# Build custom Encore CLI with your platform URL
go build -ldflags "\
  -X encr.dev/internal/conf.defaultPlatformURL=https://api.mycompany.com \
  -X encr.dev/internal/conf.defaultDevDashURL=https://devdash.mycompany.com \
" -o encore-custom ./cli/cmd/encore

# Users can now use the custom binary without environment variables
./encore-custom run
```

**Pros:**
- No code changes
- Users don't need to set environment variables
- Easy to maintain (just change build flags)

**Cons:**
- Still doesn't change hardcoded UI URLs
- Requires custom build process

---

### Strategy 3: Code Modifications (Complete Customization)

**Effort:** ~50-150 lines of code changes  
**Use Case:** Full white-labeling, complete control

## Required Code Changes

### Change 1: Add Web Dashboard URL Configuration

**File:** `internal/conf/conf.go`

**Current Code (lines 28-32):**
```go
var (
    defaultPlatformURL     = "https://api.encore.cloud"
    defaultDevDashURL      = "https://devdash.encore.dev"
    defaultConfigDirectory = "encore"
)
```

**Modified Code:**
```go
var (
    defaultPlatformURL     = "https://api.encore.cloud"
    defaultDevDashURL      = "https://devdash.encore.dev"
    defaultWebDashURL      = "https://app.encore.cloud"  // NEW
    defaultConfigDirectory = "encore"
)
```

**Add after line 53:**
```go
// WebDashURL is the base URL for the web dashboard.
var WebDashURL = (func() string {
    if u := os.Getenv("ENCORE_WEBDASH_URL"); u != "" {
        return u
    }
    return defaultWebDashURL
})()
```

**Patch Summary:**
- **Lines added:** 7
- **Lines modified:** 1
- **Total change:** 8 lines

---

### Change 2: Update Deploy Command URL

**File:** `cli/cmd/encore/deploy.go`

**Current Code (line 78):**
```go
url := fmt.Sprintf("https://app.encore.cloud/%s/deploys/%s/%s", 
    appSlug, rollout.EnvName, strings.TrimPrefix(rollout.ID, "roll_"))
```

**Modified Code:**
```go
url := fmt.Sprintf("%s/%s/deploys/%s/%s", 
    conf.WebDashURL, appSlug, rollout.EnvName, 
    strings.TrimPrefix(rollout.ID, "roll_"))
```

**Required Import:**
```go
import (
    // ... existing imports ...
    "encr.dev/internal/conf"  // ADD THIS
)
```

**Patch Summary:**
- **Lines modified:** 1
- **Imports added:** 1
- **Total change:** 2 lines

---

### Change 3: Update App Create Command URL

**File:** `cli/cmd/encore/app/create.go`

**Current Code (line 344):**
```go
fmt.Printf("Web URL:  %s%s", cyanf("https://app.encore.cloud/"+app.Slug), cmdutil.Newline)
```

**Modified Code:**
```go
fmt.Printf("Web URL:  %s%s", cyanf(conf.WebDashURL+"/"+app.Slug), cmdutil.Newline)
```

**Required Import:**
```go
import (
    // ... existing imports ...
    "encr.dev/internal/conf"  // ADD THIS
)
```

**Patch Summary:**
- **Lines modified:** 1
- **Imports added:** 1
- **Total change:** 2 lines

---

### Change 4: Update App Initialize Command URL

**File:** `cli/cmd/encore/app/initialize.go`

**Current Code (line 151):**
```go
_, _ = fmt.Fprintf(os.Stdout, "- Cloud Dashboard: %s\n\n", 
    cyan.Sprintf("https://app.encore.cloud/%s", appSlug))
```

**Modified Code:**
```go
_, _ = fmt.Fprintf(os.Stdout, "- Cloud Dashboard: %s\n\n", 
    cyan.Sprintf("%s/%s", conf.WebDashURL, appSlug))
```

**Required Import:**
```go
import (
    // ... existing imports ...
    "encr.dev/internal/conf"  // ADD THIS
)
```

**Patch Summary:**
- **Lines modified:** 1
- **Imports added:** 1
- **Total change:** 2 lines

---

### Change 5: Update Documentation URLs (Optional)

These changes are optional and only needed if you want to redirect documentation links to your own docs.

**Files to modify:**
- `cli/cmd/encore/k8s/config.go` (line 139)
- `cli/cmd/encore/telemetry.go` (line 44)
- `cli/cmd/encore/version.go` (line 69)
- `cli/daemon/export/infra_config.go` (line 23)
- `cli/daemon/mcp/docs_tools.go` (lines 171, 190)
- `cli/daemon/run/proc_groups.go` (line 365)
- `cli/internal/login/interactive.go` (line 121)
- `cli/internal/update/update.go` (lines 36, 164, 171)

**Pattern:**
Replace `https://encore.dev` with `conf.DocsURL` (after adding it to `internal/conf/conf.go`)

---

## Complete Patch File

Here's a complete Git patch you can apply:

```diff
diff --git a/internal/conf/conf.go b/internal/conf/conf.go
index 1234567..abcdefg 100644
--- a/internal/conf/conf.go
+++ b/internal/conf/conf.go
@@ -28,6 +28,7 @@ var ErrNotLoggedIn = errors.New("not logged in: run 'encore auth login' first")
 var (
 	defaultPlatformURL     = "https://api.encore.cloud"
 	defaultDevDashURL      = "https://devdash.encore.dev"
+	defaultWebDashURL      = "https://app.encore.cloud"
 	defaultConfigDirectory = "encore"
 )
 
@@ -52,6 +53,14 @@ var DevDashURL = (func() string {
 	return defaultDevDashURL
 })()
 
+// WebDashURL is the base URL for the web dashboard.
+var WebDashURL = (func() string {
+	if u := os.Getenv("ENCORE_WEBDASH_URL"); u != "" {
+		return u
+	}
+	return defaultWebDashURL
+})()
+
 // CacheDevDash reports whether or not the dev dash contents should be cached.
 var CacheDevDash = (func() bool {
 	return !strings.Contains(DevDashURL, "localhost")

diff --git a/cli/cmd/encore/deploy.go b/cli/cmd/encore/deploy.go
index 2345678..bcdefgh 100644
--- a/cli/cmd/encore/deploy.go
+++ b/cli/cmd/encore/deploy.go
@@ -12,6 +12,7 @@ import (
 
 	"encr.dev/cli/cmd/encore/cmdutil"
 	"encr.dev/cli/internal/platform"
+	"encr.dev/internal/conf"
 	"encr.dev/pkg/appfile"
 )
 
@@ -75,7 +76,7 @@ var deployAppCmd = &cobra.Command{
 		if err != nil {
 			cmdutil.Fatalf("failed to deploy: %v", err)
 		}
-		url := fmt.Sprintf("https://app.encore.cloud/%s/deploys/%s/%s", appSlug, rollout.EnvName, strings.TrimPrefix(rollout.ID, "roll_"))
+		url := fmt.Sprintf("%s/%s/deploys/%s/%s", conf.WebDashURL, appSlug, rollout.EnvName, strings.TrimPrefix(rollout.ID, "roll_"))
 		switch format.Value {
 		case "text":
 			fmt.Println(aurora.Sprintf("\n%s %s\n", aurora.Bold("Started Deploy:"), url))

diff --git a/cli/cmd/encore/app/create.go b/cli/cmd/encore/app/create.go
index 3456789..cdefghi 100644
--- a/cli/cmd/encore/app/create.go
+++ b/cli/cmd/encore/app/create.go
@@ -341,7 +341,7 @@ func createApp(params *createParams) (*platform.App, error) {
 		fmt.Printf("App ID:   %s%s", cyanf(app.Slug), cmdutil.Newline)
 		fmt.Println()
 		fmt.Printf("API URL:  %s%s", cyanf(apiURL), cmdutil.Newline)
-		fmt.Printf("Web URL:  %s%s", cyanf("https://app.encore.cloud/"+app.Slug), cmdutil.Newline)
+		fmt.Printf("Web URL:  %s%s", cyanf(conf.WebDashURL+"/"+app.Slug), cmdutil.Newline)
 		fmt.Println()
 	}
 

diff --git a/cli/cmd/encore/app/initialize.go b/cli/cmd/encore/app/initialize.go
index 4567890..defghij 100644
--- a/cli/cmd/encore/app/initialize.go
+++ b/cli/cmd/encore/app/initialize.go
@@ -148,7 +148,7 @@ func initialize(params *initializeParams) error {
 		_, _ = fmt.Fprintf(os.Stdout, "Successfully created app!\n\n")
 		_, _ = fmt.Fprintf(os.Stdout, "App ID: %s\n", cyan.Sprint(appSlug))
 		_, _ = fmt.Fprintf(os.Stdout, "- Local Dashboard: %s\n", cyan.Sprintf("http://localhost:%d/%s", devDashPort, appSlug))
-		_, _ = fmt.Fprintf(os.Stdout, "- Cloud Dashboard: %s\n\n", cyan.Sprintf("https://app.encore.cloud/%s", appSlug))
+		_, _ = fmt.Fprintf(os.Stdout, "- Cloud Dashboard: %s\n\n", cyan.Sprintf("%s/%s", conf.WebDashURL, appSlug))
 	}
 
 	return nil
```

**To apply this patch:**
```bash
cd /path/to/encore
git apply self-hosted-platform.patch
```

---

## Authentication Customization

### Current Authentication Flow

The Encore CLI uses **OAuth 2.0 Device Authorization Grant** (RFC 8628):

1. CLI calls `POST /oauth/device-auth` to get device code
2. User visits verification URL and logs in via browser
3. CLI polls `POST /oauth/token` until user completes login
4. Token stored in `~/.config/encore/.auth_token`
5. Token auto-refreshed via `POST /login/oauth:refresh-token`

### Custom Authentication Options

#### Option 1: Compatible OAuth Implementation

Implement the same OAuth endpoints on your platform:

**Required Endpoints:**
```
POST /oauth/device-auth
POST /oauth/token
POST /login/oauth:refresh-token
```

**No code changes needed** - just set `ENCORE_PLATFORM_API_URL`

#### Option 2: API Key Authentication

Replace OAuth with simple API key authentication.

**File:** `cli/internal/platform/login.go`

**Add new function:**
```go
func LoginWithAPIKey(ctx context.Context, apiKey string) (*OAuthData, error) {
    var resp OAuthData
    err := call(ctx, "POST", "/login/api-key", 
        map[string]string{"api_key": apiKey}, &resp, false)
    return &resp, err
}
```

**File:** `cli/cmd/encore/auth.go` (create new file)

```go
package main

import (
    "github.com/spf13/cobra"
    "encr.dev/cli/cmd/encore/cmdutil"
    "encr.dev/cli/internal/platform"
    "encr.dev/internal/conf"
)

var apiKey string

var loginAPIKeyCmd = &cobra.Command{
    Use:   "login-apikey",
    Short: "Login using API key",
    Run: func(c *cobra.Command, args []string) {
        data, err := platform.LoginWithAPIKey(c.Context(), apiKey)
        if err != nil {
            cmdutil.Fatalf("login failed: %v", err)
        }
        
        cfg := &conf.Config{
            Token:   *data.Token,
            Actor:   data.Actor,
            Email:   data.Email,
            AppSlug: data.AppSlug,
        }
        
        if err := conf.Write(cfg); err != nil {
            cmdutil.Fatalf("failed to save config: %v", err)
        }
        
        fmt.Println("Successfully logged in!")
    },
}

func init() {
    authCmd.AddCommand(loginAPIKeyCmd)
    loginAPIKeyCmd.Flags().StringVar(&apiKey, "key", "", "API key")
    _ = loginAPIKeyCmd.MarkFlagRequired("key")
}
```

**Patch Summary:**
- **New files:** 1 (`cli/cmd/encore/auth.go`)
- **Modified files:** 1 (`cli/internal/platform/login.go`)
- **Total lines added:** ~50

#### Option 3: Disable Authentication

For internal use where authentication isn't needed.

**File:** `internal/conf/conf.go`

**Modify `Token()` method (line 225):**
```go
func (ts *TokenSource) Token() (*oauth2.Token, error) {
    // Skip authentication for self-hosted
    if os.Getenv("ENCORE_SKIP_AUTH") != "" {
        return &oauth2.Token{
            AccessToken: "local-dev-token",
            TokenType:   "Bearer",
        }, nil
    }
    
    // ... existing code ...
}
```

**Usage:**
```bash
export ENCORE_SKIP_AUTH=1
encore run  # No login required
```

---

## Build Instructions

### Building Custom CLI

```bash
# Clone the repository
git clone https://github.com/encoredev/encore.git
cd encore

# Apply your patches
git apply /path/to/your-patches.patch

# Build the CLI
go build -o encore-custom \
  -ldflags "\
    -X encr.dev/internal/conf.defaultPlatformURL=https://api.mycompany.com \
    -X encr.dev/internal/conf.defaultDevDashURL=https://devdash.mycompany.com \
    -X encr.dev/internal/conf.defaultWebDashURL=https://app.mycompany.com \
  " \
  ./cli/cmd/encore

# Test the custom CLI
./encore-custom version
```

### Automated Build Script

**File:** `build-custom.sh`

```bash
#!/bin/bash
set -e

# Configuration
PLATFORM_API_URL="${PLATFORM_API_URL:-https://api.mycompany.com}"
DEVDASH_URL="${DEVDASH_URL:-https://devdash.mycompany.com}"
WEBDASH_URL="${WEBDASH_URL:-https://app.mycompany.com}"
OUTPUT_NAME="${OUTPUT_NAME:-encore-custom}"

echo "Building custom Encore CLI..."
echo "  Platform API: $PLATFORM_API_URL"
echo "  Dev Dashboard: $DEVDASH_URL"
echo "  Web Dashboard: $WEBDASH_URL"

go build -o "$OUTPUT_NAME" \
  -ldflags "\
    -X encr.dev/internal/conf.defaultPlatformURL=$PLATFORM_API_URL \
    -X encr.dev/internal/conf.defaultDevDashURL=$DEVDASH_URL \
    -X encr.dev/internal/conf.defaultWebDashURL=$WEBDASH_URL \
  " \
  ./cli/cmd/encore

echo "Build complete: $OUTPUT_NAME"
./"$OUTPUT_NAME" version
```

**Usage:**
```bash
chmod +x build-custom.sh
PLATFORM_API_URL=https://api.example.com ./build-custom.sh
```

---

## Testing Your Changes

### Test Checklist

- [ ] `encore auth login` - Authentication works
- [ ] `encore run` - Local development works
- [ ] `encore deploy` - Deployment URL points to your dashboard
- [ ] `encore app create` - App creation URL is correct
- [ ] `encore gen client` - Client generation works
- [ ] `encore secret set` - Secret management works
- [ ] Database proxy connects to your platform
- [ ] Log streaming works

### Mock Platform Server

For testing, you can create a simple mock server:

**File:** `mock-platform-server.go`

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/oauth/device-auth", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "device_code":      "test-device-code",
            "user_code":        "TEST-CODE",
            "verification_uri": "http://localhost:8080/verify",
            "expires_in":       300,
            "interval":         5,
        })
    })
    
    http.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(map[string]interface{}{
            "access_token":  "test-access-token",
            "token_type":    "Bearer",
            "refresh_token": "test-refresh-token",
            "actor":         "user_test",
            "email":         "test@example.com",
        })
    })
    
    http.HandleFunc("/user/apps", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode([]map[string]string{
            {"id": "app1", "slug": "test-app", "name": "Test App"},
        })
    })
    
    fmt.Println("Mock platform server running on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

**Run the mock server:**
```bash
go run mock-platform-server.go
```

**Test with custom CLI:**
```bash
export ENCORE_PLATFORM_API_URL=http://localhost:8080
encore auth login
```

---

## Summary of Changes

### Minimal Changes (Recommended)

**Total Lines Changed:** ~15 lines

| File | Lines Added | Lines Modified | Purpose |
|------|-------------|----------------|---------|
| `internal/conf/conf.go` | 8 | 1 | Add WebDashURL configuration |
| `cli/cmd/encore/deploy.go` | 1 | 1 | Use configurable dashboard URL |
| `cli/cmd/encore/app/create.go` | 1 | 1 | Use configurable dashboard URL |
| `cli/cmd/encore/app/initialize.go` | 1 | 1 | Use configurable dashboard URL |

### With Custom Authentication

**Total Lines Changed:** ~65 lines

- Minimal changes (above): 15 lines
- Custom auth implementation: 50 lines

### Complete White-Labeling

**Total Lines Changed:** ~150 lines

- Minimal changes: 15 lines
- Custom auth: 50 lines
- Documentation URL updates: 85 lines

---

## Maintenance Strategy

### Keeping Up with Upstream

To stay current with Encore updates:

1. **Use Git branches:**
   ```bash
   git checkout -b custom-platform
   # Apply your patches
   git commit -am "Custom platform configuration"
   ```

2. **Rebase on updates:**
   ```bash
   git fetch upstream
   git rebase upstream/main
   # Resolve conflicts if any
   ```

3. **Automated testing:**
   - Set up CI/CD to build custom CLI
   - Run integration tests against your platform
   - Alert on build failures

### Recommended Approach

**For most use cases, use Strategy 2 (Build-Time Linker Flags):**

✅ No code changes  
✅ Easy to maintain  
✅ Works with upstream updates  
✅ Simple build process  

**Only modify code if you need:**
- Custom authentication mechanism
- White-labeled UI messages
- Custom documentation links

---

## Conclusion

The Encore CLI is well-architected for self-hosting with minimal modifications required. The recommended approach is:

1. **Start with environment variables** for testing
2. **Use build-time linker flags** for distribution
3. **Only modify code** if you need custom authentication or white-labeling

**Key Takeaway:** You can redirect all API calls to your self-hosted platform with **zero code changes** by using `ENCORE_PLATFORM_API_URL`. The ~15 lines of code changes are only needed to update UI messages with your custom dashboard URLs.
