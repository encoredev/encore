# Rencore - Self-Hosted Encore CLI

Rencore is a custom build of the [Encore CLI](https://github.com/encoredev/encore) configured for self-hosted platform deployments.

## Overview

This repository maintains a fork of the official Encore CLI with patches applied to redirect all cloud API calls and dashboard URLs to a custom, self-hosted platform.

**Key Features:**
- ✅ All API calls redirect to custom platform URL
- ✅ Custom web dashboard URLs in CLI output
- ✅ Custom documentation URLs
- ✅ API key authentication support
- ✅ Fully automated release process
- ✅ Homebrew tap for easy installation

## Architecture

### Patch-Based Approach

We use a **patch bundle model** to maintain our customizations:

```
┌─────────────────────────────────────────────┐
│  upstream.ref                               │
│  (pinned upstream commit SHA)               │
└──────────────┬──────────────────────────────┘
               │
               │ git checkout
               ▼
┌─────────────────────────────────────────────┐
│  Clean upstream codebase                    │
└──────────────┬──────────────────────────────┘
               │
               │ apply patches
               ▼
┌─────────────────────────────────────────────┐
│  patches/0001-*.patch                       │
│  patches/0002-*.patch                       │
│  patches/0003-*.patch                       │
│  patches/0004-*.patch                       │
└──────────────┬──────────────────────────────┘
               │
               │ build
               ▼
┌─────────────────────────────────────────────┐
│  Rencore binary with custom URLs           │
└─────────────────────────────────────────────┘
```

### Repository Structure

```
encore/
├── upstream.ref              # Pinned upstream commit
├── patches/                  # Our customization patches
│   ├── 0001-add-urlutil-package.patch
│   ├── 0002-self-hosted-platform-urls.patch
│   ├── 0003-add-api-key-authentication.patch
│   └── 0004-update-docs-urls.patch
├── scripts/
│   ├── apply_patches.sh      # Apply all patches
│   └── build.sh              # Build Rencore binary
└── .github/workflows/
    └── rencore-release.yml   # Automated release workflow
```

## Patches Applied

### 0001: URL Utility Package
Adds `internal/urlutil` package with safe URL joining functions.

### 0002: Self-Hosted Platform URLs
Updates core configuration in `internal/conf/conf.go`:
- Adds `defaultWebDashURL` and `defaultDocsURL`
- Adds `WebDashBaseURL()` and `DocsBaseURL()` functions
- Updates commands to use configurable URLs:
  - `encore app create`
  - `encore app initialize`
  - `encore deploy`

### 0003: API Key Authentication
Adds alternative authentication method:
- New command: `encore auth login-apikey --auth-key=<KEY>`
- Supports stdin input for CI/CD pipelines
- Bypasses OAuth flow for headless environments

### 0004: Documentation URLs
Updates hardcoded documentation URLs in:
- `cli/cmd/encore/auth/auth.go`
- `cli/cmd/encore/k8s/config.go`
- `cli/cmd/encore/telemetry.go`
- `cli/cmd/encore/version.go`
- `cli/daemon/export/infra_config.go`
- `cli/daemon/mcp/docs_tools.go`
- `cli/daemon/run/proc_groups.go`

## Configuration

Rencore can be configured via environment variables or build-time ldflags.

### Environment Variables

```bash
export ENCORE_PLATFORM_API_URL="https://api.stagecraft.ing"
export ENCORE_WEBDASH_URL="https://app.stagecraft.ing"
export ENCORE_DEVDASH_URL="https://devdash.stagecraft.ing"
export ENCORE_DOCS_URL="https://docs.stagecraft.ing"
```

### Build-Time Configuration

URLs are baked into the binary during build:

```bash
go build -ldflags "\
  -X encr.dev/internal/conf.defaultPlatformURL=https://api.stagecraft.ing \
  -X encr.dev/internal/conf.defaultWebDashURL=https://app.stagecraft.ing \
  -X encr.dev/internal/conf.defaultDevDashURL=https://devdash.stagecraft.ing \
  -X encr.dev/internal/conf.defaultDocsURL=https://docs.stagecraft.ing \
" -o encore ./cli/cmd/encore
```

## Installation

### Homebrew (Recommended for macOS)

```bash
# Add the tap
brew tap stagecraft-ing/tap

# Install Rencore
brew install rencore

# Switch from official Encore to Rencore
brew unlink encore || true
brew link --overwrite rencore

# Verify
encore version
```

### Manual Installation

#### macOS (Apple Silicon)
```bash
VERSION=v1.44.7
curl -Lo encore.tar.gz "https://github.com/stagecraft-ing/encore/releases/download/$VERSION/encore-$VERSION-darwin_arm64.tar.gz"
tar -xzf encore.tar.gz
sudo mv encore /usr/local/bin/
```

#### macOS (Intel)
```bash
VERSION=v1.44.7
curl -Lo encore.tar.gz "https://github.com/stagecraft-ing/encore/releases/download/$VERSION/encore-$VERSION-darwin_amd64.tar.gz"
tar -xzf encore.tar.gz
sudo mv encore /usr/local/bin/
```

#### Linux (x86_64)
```bash
VERSION=v1.44.7
curl -Lo encore.tar.gz "https://github.com/stagecraft-ing/encore/releases/download/$VERSION/encore-$VERSION-linux_amd64.tar.gz"
tar -xzf encore.tar.gz
sudo mv encore /usr/local/bin/
```

#### Linux (ARM64)
```bash
VERSION=v1.44.7
curl -Lo encore.tar.gz "https://github.com/stagecraft-ing/encore/releases/download/$VERSION/encore-$VERSION-linux_arm64.tar.gz"
tar -xzf encore.tar.gz
sudo mv encore /usr/local/bin/
```

## Switching Between Official and Custom

### Use Rencore
```bash
brew unlink encore || true
brew link --overwrite rencore
```

### Use Official Encore
```bash
brew unlink rencore
brew link --overwrite encore
```

## Development

### Prerequisites

- Go 1.21+
- Git
- Make (optional)

### Building Locally

```bash
# Clone the repository
git clone git@github.com:stagecraft-ing/encore.git
cd encore

# Apply patches
./scripts/apply_patches.sh

# Build
./scripts/build.sh --version v1.44.7
```

### Creating a New Release

Releases are fully automated via GitHub Actions:

1. Go to Actions → Rencore Release
2. Click "Run workflow"
3. Enter version (e.g., `v1.44.7`)
4. (Optional) Customize URLs
5. Click "Run workflow"

The workflow will:
- ✅ Apply all patches
- ✅ Build for all platforms (darwin/linux, amd64/arm64)
- ✅ Generate SHA256 checksums
- ✅ Create GitHub release
- ✅ Trigger Homebrew formula update

### Updating Patches

If you need to update the patches:

```bash
# Make your changes to the code
# Then regenerate patches:

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

### Updating Upstream Reference

To update to a newer version of Encore:

```bash
# Fetch latest upstream
git remote add upstream https://github.com/encoredev/encore.git
git fetch upstream

# Update to specific commit or tag
git checkout upstream/main  # or specific commit
COMMIT=$(git rev-parse HEAD)

# Update the reference file
echo $COMMIT > upstream.ref

# Test patches apply cleanly
./scripts/apply_patches.sh

# If patches fail, resolve conflicts and regenerate patches
```

## Authentication

### OAuth (Default)
```bash
encore auth login
```

### API Key (Headless/CI)
```bash
# Via flag
encore auth login-apikey --auth-key=your-api-key

# Via stdin (for CI/CD)
echo "your-api-key" | encore auth login-apikey

# Via environment
export ENCORE_AUTH_KEY=your-api-key
echo $ENCORE_AUTH_KEY | encore auth login-apikey
```

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ENCORE_PLATFORM_API_URL` | `https://api.encore.cloud` | Platform API base URL |
| `ENCORE_WEBDASH_URL` | `https://app.encore.cloud` | Web dashboard URL |
| `ENCORE_DEVDASH_URL` | `https://devdash.encore.dev` | Local dev dashboard URL |
| `ENCORE_DOCS_URL` | `https://encore.dev` | Documentation URL |
| `ENCORE_CONFIG_DIR` | `~/.config/encore` | Config directory |
| `ENCORE_CACHE_DIR` | `~/.cache/encore/cache` | Cache directory |
| `ENCORE_DATA_DIR` | `~/.cache/encore/data` | Data directory |

### Build-Time Variables (ldflags)

| Variable | Description |
|----------|-------------|
| `encr.dev/internal/conf.defaultPlatformURL` | Default platform API URL |
| `encr.dev/internal/conf.defaultWebDashURL` | Default web dashboard URL |
| `encr.dev/internal/conf.defaultDevDashURL` | Default dev dashboard URL |
| `encr.dev/internal/conf.defaultDocsURL` | Default documentation URL |

## Maintenance

### Upstream Sync Schedule

We recommend syncing with upstream monthly or when critical security patches are released.

### Security Updates

For urgent security updates:

1. Update `upstream.ref` to the patched commit
2. Verify patches apply cleanly
3. Create a patch release (e.g., `v1.44.7-security.1`)
4. Trigger release workflow

### Breaking Changes

If upstream introduces breaking changes that conflict with our patches:

1. Update patches to resolve conflicts
2. Test thoroughly
3. Document breaking changes in release notes
4. Bump minor version (e.g., `v1.44.0` → `v1.45.0`)

## Troubleshooting

### Patches Won't Apply

```bash
# Check current HEAD vs upstream.ref
git rev-parse HEAD
cat upstream.ref

# Checkout the pinned commit
git checkout $(cat upstream.ref)

# Try applying patches again
./scripts/apply_patches.sh
```

### Build Failures

```bash
# Clean build
rm -rf dist/

# Verify Go version
go version  # Should be 1.21+

# Build with verbose output
./scripts/build.sh --version v1.44.7 2>&1 | tee build.log
```

### Formula Installation Issues

```bash
# Uninstall and reinstall
brew uninstall rencore
brew untap stagecraft-ing/tap
brew tap stagecraft-ing/tap
brew install rencore
```

## Contributing

We welcome contributions! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Update patches if needed
5. Submit a pull request

## Support

- **Issues**: https://github.com/stagecraft-ing/encore/issues
- **Discussions**: https://github.com/stagecraft-ing/encore/discussions
- **Upstream**: https://github.com/encoredev/encore

## License

Rencore inherits the license from the upstream Encore project.

See [LICENSE](LICENSE) for details.

## Acknowledgments

- **Encore Team** - For creating the excellent Encore framework
- **Community** - For feedback and contributions

## Related Resources

- [Official Encore Documentation](https://encore.dev/docs)
- [Encore GitHub Repository](https://github.com/encoredev/encore)
- [Homebrew Tap Repository](https://github.com/stagecraft-ing/homebrew-tap)
