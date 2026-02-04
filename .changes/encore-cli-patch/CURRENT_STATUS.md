# Current Implementation Status

**Last Updated**: February 4, 2026
**Status**: ✅ Production Ready

## Overview

Rencore is a custom build of the Encore CLI configured for self-hosted platform deployments. This document describes the current implementation status.

## Implementation Approach

### Patch Bundle Model ✅

We use a **patch-based approach** (Option B from planning) rather than direct code modifications:

```
upstream.ref → Clean Checkout → Apply Patches → Build → Release
```

**Benefits:**
- ✅ Clean separation from upstream
- ✅ Easy upstream sync (just update `upstream.ref`)
- ✅ No merge conflicts
- ✅ Reproducible builds
- ✅ Automated via GitHub Actions

## Current Features

### 1. Self-Hosted Platform URLs ✅

**Configuration Points:**
```go
// internal/conf/conf.go
defaultPlatformURL     = "https://api.encore.cloud"      // Can override via ENCORE_PLATFORM_API_URL
defaultWebDashURL      = "https://app.encore.cloud"      // Can override via ENCORE_WEBDASH_URL
defaultDevDashURL      = "https://devdash.encore.dev"    // Can override via ENCORE_DEVDASH_URL
defaultDocsURL         = "https://encore.dev"            // Can override via ENCORE_DOCS_URL
```

**Runtime Override:**
```bash
export ENCORE_PLATFORM_API_URL="https://api.stagecraft.ing"
export ENCORE_WEBDASH_URL="https://app.stagecraft.ing"
export ENCORE_DEVDASH_URL="https://devdash.stagecraft.ing"
export ENCORE_DOCS_URL="https://docs.stagecraft.ing"
```

**Build-Time Override:**
```bash
go build -ldflags "\
  -X encr.dev/internal/conf.defaultPlatformURL=https://api.stagecraft.ing \
  -X encr.dev/internal/conf.defaultWebDashURL=https://app.stagecraft.ing \
" -o encore ./cli/cmd/encore
```

**Affected Commands:**
- `encore deploy` - Shows custom dashboard URL for deployment
- `encore app create` - Shows custom web URL
- `encore app initialize` - Shows custom cloud dashboard URL
- All documentation links point to custom docs

### 2. API Key Authentication ✅

**New Command:**
```bash
encore auth login-apikey --auth-key=<KEY>
```

**Features:**
- ✅ Stdin support for CI/CD: `echo $KEY | encore auth login-apikey`
- ✅ Seamless integration with existing auth system
- ✅ Standard config storage (`~/.config/encore/.auth_token`)

**Implementation:**
- `cli/cmd/encore/auth/apikey.go` - New file with login-apikey command
- `cli/internal/login/login.go` - Added `WithAuthKey()` function

### 3. URL Utility Package ✅

**New Package:** `internal/urlutil`

**Purpose:** Safe URL concatenation to avoid double slashes and handle edge cases

**Function:**
```go
func JoinURL(base, relPath string) string
```

**Features:**
- ✅ Guards against double slashes
- ✅ Handles full URLs in relPath (returns as-is)
- ✅ Handles empty base URL
- ✅ Comprehensive test coverage

### 4. Automated Release Process ✅

**GitHub Actions Workflow:** `.github/workflows/rencore-release.yml`

**Features:**
- ✅ Multi-platform builds: darwin/linux, amd64/arm64
- ✅ Automated patch application
- ✅ SHA256 checksum generation
- ✅ GitHub release creation
- ✅ Homebrew formula update trigger
- ✅ Configurable URLs via workflow inputs

**Artifacts Produced:**
- `encore-vX.Y.Z-darwin_amd64.tar.gz` + `.sha256`
- `encore-vX.Y.Z-darwin_arm64.tar.gz` + `.sha256`
- `encore-vX.Y.Z-linux_amd64.tar.gz` + `.sha256`
- `encore-vX.Y.Z-linux_arm64.tar.gz` + `.sha256`

### 5. Homebrew Distribution ✅

**Formula:** `stagecraft-ing/homebrew-tap/Formula/rencore.rb`

**Features:**
- ✅ Keg-only installation (no auto-link)
- ✅ Easy switching: `brew link --overwrite rencore`
- ✅ Auto-update on release
- ✅ Multi-platform support

**Installation:**
```bash
brew tap stagecraft-ing/tap
brew install rencore
brew link --overwrite rencore
```

## Patches Applied

### Patch 1: URL Utility Package (1,802 bytes)
**File:** `patches/0001-add-urlutil-package.patch`

**Creates:**
- `internal/urlutil/join.go` - JoinURL function
- `internal/urlutil/join_test.go` - Test coverage

### Patch 2: Self-Hosted Platform URLs (3,735 bytes)
**File:** `patches/0002-self-hosted-platform-urls.patch`

**Modifies:**
- `internal/conf/conf.go` - Adds WebDashBaseURL() and DocsBaseURL()
- `cli/cmd/encore/deploy.go` - Uses configurable URL
- `cli/cmd/encore/app/create.go` - Uses configurable URL
- `cli/cmd/encore/app/initialize.go` - Uses configurable URL

### Patch 3: API Key Authentication (3,002 bytes)
**File:** `patches/0003-add-api-key-authentication.patch`

**Creates:**
- `cli/cmd/encore/auth/apikey.go` - login-apikey command
- `internal/conf/conf_custom_test.go` - Config tests

**Modifies:**
- `cli/internal/login/login.go` - Adds WithAuthKey() function

### Patch 4: Documentation URLs (6,591 bytes)
**File:** `patches/0004-update-docs-urls.patch`

**Modifies:**
- `cli/cmd/encore/auth/auth.go`
- `cli/cmd/encore/k8s/config.go`
- `cli/cmd/encore/telemetry.go`
- `cli/cmd/encore/version.go`
- `cli/daemon/export/infra_config.go`
- `cli/daemon/mcp/docs_tools.go`
- `cli/daemon/run/proc_groups.go`

## Upstream Reference

**Pinned Commit:** `73b94e4d2565489595fa5b7cd231efc44c9cd599`

**Upstream Repository:** https://github.com/encoredev/encore

**Commit Message:** "Add docs for GET /rollouts/{id} platform API endpoint (#2263)"

**Date:** Latest as of February 4, 2026

## Files Created/Modified

### New Files
```
upstream.ref                              # Upstream commit reference
patches/                                  # Patch files (4 files)
scripts/apply_patches.sh                  # Patch application script
scripts/build.sh                          # Build script
.github/workflows/rencore-release.yml     # Release automation
internal/urlutil/join.go                  # URL utility
internal/urlutil/join_test.go             # URL utility tests
cli/cmd/encore/auth/apikey.go             # API key auth command
internal/conf/conf_custom_test.go         # Config tests
RENCORE.md                                # Main documentation
QUICKSTART.md                             # Quick start guide
DEPLOYMENT_CHECKLIST.md                   # Deployment guide
RELEASES.md                               # Release process
HOMEBREW_TAP_README.md                    # Homebrew setup
IMPLEMENTATION_SUMMARY.md                 # Implementation overview
.changes/encore-cli-patch/README.md       # This directory index
.changes/encore-cli-patch/CURRENT_STATUS.md        # This file
.changes/encore-cli-patch/MIGRATION_GUIDE.md       # Migration guide
```

### Modified Files (via Patches)
```
internal/conf/conf.go                     # Added WebDashBaseURL(), DocsBaseURL()
cli/cmd/encore/deploy.go                  # Custom dashboard URLs
cli/cmd/encore/app/create.go              # Custom dashboard URLs
cli/cmd/encore/app/initialize.go          # Custom dashboard URLs
cli/cmd/encore/auth/auth.go               # Documentation URLs
cli/cmd/encore/k8s/config.go              # Documentation URLs
cli/cmd/encore/telemetry.go               # Documentation URLs
cli/cmd/encore/version.go                 # Documentation URLs
cli/daemon/export/infra_config.go         # Documentation URLs
cli/daemon/mcp/docs_tools.go              # Documentation URLs
cli/daemon/run/proc_groups.go             # Documentation URLs
cli/internal/login/login.go               # API key auth support
```

## Build and Test

### Apply Patches
```bash
./scripts/apply_patches.sh
```

### Build Binary
```bash
./scripts/build.sh --version v1.44.7
```

### Test Locally
```bash
encore version
encore auth login-apikey --help
encore app create --help
```

## Maintenance

### Updating Upstream

1. **Fetch latest upstream:**
   ```bash
   git fetch upstream
   ```

2. **Checkout new version:**
   ```bash
   git checkout upstream/main  # or specific tag
   ```

3. **Update reference:**
   ```bash
   git rev-parse HEAD > upstream.ref
   ```

4. **Test patches:**
   ```bash
   ./scripts/apply_patches.sh
   ```

5. **Resolve conflicts if any:**
   - If patches fail, regenerate them
   - Test thoroughly
   - Update patch files

### Regenerating Patches

If upstream changes conflict:

```bash
# Make your changes
# Then regenerate patch files

# For urlutil
git add internal/urlutil/
git diff --cached internal/urlutil/ > patches/0001-add-urlutil-package.patch
git reset HEAD internal/urlutil/

# For self-hosted URLs
git diff cli/cmd/encore/app/create.go \
         cli/cmd/encore/app/initialize.go \
         cli/cmd/encore/deploy.go \
         internal/conf/conf.go > patches/0002-self-hosted-platform-urls.patch

# For API key auth
git add cli/cmd/encore/auth/apikey.go internal/conf/conf_custom_test.go
git diff --cached > patches/0003-add-api-key-authentication.patch
git reset HEAD cli/cmd/encore/auth/apikey.go internal/conf/conf_custom_test.go

# For docs URLs
git diff cli/cmd/encore/auth/auth.go \
         cli/cmd/encore/k8s/config.go \
         cli/cmd/encore/telemetry.go \
         cli/cmd/encore/version.go \
         cli/daemon/export/infra_config.go \
         cli/daemon/mcp/docs_tools.go \
         cli/daemon/run/proc_groups.go > patches/0004-update-docs-urls.patch
```

## Known Limitations

1. **Windows Builds:** Built but not distributed via Homebrew (Homebrew is macOS/Linux only)
2. **Code Signing:** Not implemented (consider for macOS notarization)
3. **NPM Publishing:** Disabled in build process
4. **HTTPS Local Dev:** Not implemented (see `./refrence/local-https-flag-feasibility-and-implementation-plan.md``)

## Future Enhancements

### Phase 2 (Planned)
- [ ] Implement local HTTPS support
- [ ] Add Windows package manager support (Chocolatey/Scoop)
- [ ] Implement code signing and notarization
- [ ] Add automated tests for patches
- [ ] Create beta/nightly release channels

### Phase 3 (Ideas)
- [ ] Custom branding (logo, colors)
- [ ] Telemetry configuration
- [ ] Plugin system for custom commands
- [ ] Multi-platform self-hosted backend

## Success Metrics

| Metric | Status |
|--------|--------|
| Patches apply cleanly | ✅ Yes |
| Build succeeds | ✅ Yes |
| Multi-platform support | ✅ Yes (4 platforms) |
| GitHub Actions working | ✅ Yes |
| Homebrew formula ready | ✅ Yes |
| Documentation complete | ✅ Yes |
| First release | ⏳ Pending deployment |
| User testing | ⏳ Pending deployment |

## Support

- **Issues**: https://github.com/stagecraft-ing/encore/issues
- **Discussions**: https://github.com/stagecraft-ing/encore/discussions
- **Documentation**: See root-level `.md` files

---

**Status Legend:**
- ✅ Complete and tested
- ⏳ Ready but pending deployment
- ⚠️ In progress
- ❌ Not started
- 🔜 Planned for future

**Next Steps:** Follow [DEPLOYMENT_CHECKLIST.md](../../DEPLOYMENT_CHECKLIST.md) to deploy.
