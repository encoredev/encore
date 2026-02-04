# Release Process

This document describes the release process for Rencore.

## Release Types

### Stable Releases

Format: `vMAJOR.MINOR.PATCH` (e.g., `v1.54.0`)

- Based on stable upstream Encore releases
- Thoroughly tested
- Recommended for production use
- Published to Homebrew tap as `rencore`

### Beta Releases

Format: `vMAJOR.MINOR.PATCH-beta.N` (e.g., `v1.54.0-beta.1`)

- Based on upstream beta releases
- Test new features before stable
- Published to Homebrew tap as `rencore-beta` (optional)

### Nightly Releases

Format: `vMAJOR.MINOR.PATCH-nightly.YYYYMMDD` (e.g., `v1.54.0-nightly.20260204`)

- Built from latest upstream main branch
- Bleeding edge features
- Not recommended for production
- Published to Homebrew tap as `rencore-nightly` (optional)

### Patch Releases

Format: `vMAJOR.MINOR.PATCH-patch.N` (e.g., `v1.54.0-patch.1`)

- Custom patches or hotfixes
- Independent of upstream releases
- For urgent fixes to our customizations

## Creating a Release

### Automated Release (Recommended)

1. **Navigate to GitHub Actions**
   - Go to https://github.com/stagecraft-ing/encore/actions
   - Select "Rencore Release" workflow

2. **Click "Run workflow"**

3. **Fill in the form:**
   - **Version**: e.g., `v1.54.0`
   - **Platform API URL**: (default: `https://api.stagecraft.ing`)
   - **Web Dashboard URL**: (default: `https://app.stagecraft.ing`)
   - **Dev Dashboard URL**: (default: `https://devdash.stagecraft.ing`)
   - **Documentation URL**: (default: `https://docs.stagecraft.ing`)

Platform API	https://api.encore.dev	             https://api.stagecraft.ing
Web Dashboard	https://app.encore.dev	             https://app.stagecraft.ing
Dev Dashboard	https://devdash.encore.dev	     https://devdash.stagecraft.ing
Documentation	https://encore.dev/docs	             https://docs.stagecraft.ing

4. **Click "Run workflow"**

5. **Monitor the build**
   - Build takes ~15-30 minutes
   - Builds for 4 platforms: darwin_amd64, darwin_arm64, linux_amd64, linux_arm64
   - Automatically creates GitHub release
   - Automatically updates Homebrew formula

### Manual Release (Advanced)

If you need to build locally:

```bash
# 1. Checkout the repository
git clone git@github.com:stagecraft-ing/encore.git
cd encore

# 2. Ensure you're on the correct commit
git checkout $(cat upstream.ref)

# 3. Apply patches
./scripts/apply_patches.sh

# 4. Build all platforms
./scripts/build.sh --version v1.54.0

# 5. Verify artifacts
ls -lh dist/artifacts/

# 6. Create release manually on GitHub
# - Go to https://github.com/stagecraft-ing/encore/releases/new
# - Tag: v1.54.0
# - Upload artifacts from dist/artifacts/
# - Write release notes (see template below)

# 7. Update Homebrew formula manually
# - Clone stagecraft-ing/homebrew-tap
# - Update Formula/rencore.rb
# - Commit and push
```

## Release Checklist

Before creating a release:

- [ ] Verify `upstream.ref` points to the correct commit
- [ ] All patches apply cleanly (`./scripts/apply_patches.sh`)
- [ ] Local build succeeds (`./scripts/build.sh`)
- [ ] Tests pass (if any)
- [ ] Version follows semantic versioning
- [ ] Release notes are prepared
- [ ] Changelog is updated

After release:

- [ ] GitHub release created successfully
- [ ] All 4 platform artifacts uploaded
- [ ] SHA256 checksums generated
- [ ] Homebrew formula updated
- [ ] Release announced (Slack, Discord, email, etc.)
- [ ] Documentation updated if needed

## Release Notes Template

```markdown
# Rencore v1.54.0

Custom Encore CLI build for self-hosted platform.

## Changes

- Based on [Encore v1.54.0](https://github.com/encoredev/encore/releases/tag/v1.54.0)
- Applied custom patches for self-hosted platform support

## Configuration

- **Platform API**: https://api.stagecraft.ing
- **Web Dashboard**: https://app.stagecraft.ing
- **Dev Dashboard**: https://devdash.stagecraft.ing
- **Documentation**: https://docs.stagecraft.ing

Those stagecraft.ing URLs you listed are your replacements. The originals in upstream Encore are the following canonical endpoints.

Original Encore configuration (upstream)

1:1 mapping 

Purpose		    Original                             Replacement                      
Platform API	https://api.encore.dev	             https://api.stagecraft.ing
Web Dashboard	https://app.encore.dev	             https://app.stagecraft.ing
Dev Dashboard	https://devdash.encore.dev	     https://devdash.stagecraft.ing
Documentation	https://encore.dev/docs	             https://docs.stagecraft.ing
    


## Installation

### Homebrew (macOS/Linux)

```bash
brew tap stagecraft-ing/tap
brew install rencore

# Switch from official Encore
brew unlink encore || true
brew link --overwrite rencore
```

### Manual Installation

**macOS (Apple Silicon)**
```bash
curl -Lo encore.tar.gz https://github.com/stagecraft-ing/encore/releases/download/v1.54.0/encore-v1.54.0-darwin_arm64.tar.gz
tar -xzf encore.tar.gz && sudo mv encore /usr/local/bin/
```

**macOS (Intel)**
```bash
curl -Lo encore.tar.gz https://github.com/stagecraft-ing/encore/releases/download/v1.54.0/encore-v1.54.0-darwin_amd64.tar.gz
tar -xzf encore.tar.gz && sudo mv encore /usr/local/bin/
```

**Linux (x86_64)**
```bash
curl -Lo encore.tar.gz https://github.com/stagecraft-ing/encore/releases/download/v1.54.0/encore-v1.54.0-linux_amd64.tar.gz
tar -xzf encore.tar.gz && sudo mv encore /usr/local/bin/
```

**Linux (ARM64)**
```bash
curl -Lo encore.tar.gz https://github.com/stagecraft-ing/encore/releases/download/v1.54.0/encore-v1.54.0-linux_arm64.tar.gz
tar -xzf encore.tar.gz && sudo mv encore /usr/local/bin/
```

## Upstream

Based on [encoredev/encore@73b94e4](https://github.com/encoredev/encore/commit/73b94e4d2565489595fa5b7cd231efc44c9cd599)

## Patches Applied

- `0001-add-urlutil-package.patch` - URL utility functions
- `0002-self-hosted-platform-urls.patch` - Custom platform URLs
- `0003-add-api-key-authentication.patch` - API key auth support
- `0004-update-docs-urls.patch` - Custom documentation URLs

## Full Changelog

See [CHANGELOG.md](https://github.com/stagecraft-ing/encore/blob/main/CHANGELOG.md)
```

## Versioning Strategy

We follow the upstream Encore version numbers with our own release cadence:

```
v1.54.0        ← Stable release (based on upstream v1.54.0)
v1.54.0-patch.1 ← Our patch on top of 1.54.0
v1.54.0-beta.1  ← Beta release (based on upstream beta)
v1.54.1        ← Next stable release
```

### When to Release

- **Follow upstream stable releases**: Create a Rencore release within 1-2 days
- **Security patches**: Release immediately
- **Custom patch fixes**: Release as patch versions
- **Monthly sync**: Even if no upstream release, sync patches monthly

## Hotfix Process

If urgent fixes are needed:

1. **Create a hotfix branch**
   ```bash
   git checkout -b hotfix/v1.54.0-patch.1
   ```

2. **Make the fix and update patches**
   ```bash
   # Make changes
   # Regenerate patches
   ./scripts/apply_patches.sh
   ```

3. **Test thoroughly**
   ```bash
   ./scripts/build.sh --version v1.54.0-patch.1
   ```

4. **Create PR and merge**

5. **Trigger release workflow**
   - Use version `v1.54.0-patch.1`

## Rollback Procedure

If a release has critical issues:

1. **Mark release as draft** on GitHub
2. **Revert Homebrew formula** to previous version
3. **Announce rollback** to users
4. **Fix issues** in a new release
5. **Deprecate problematic release** in notes

```bash
# Revert Homebrew formula
cd homebrew-tap
git revert HEAD
git push
```

## Testing Releases

Before announcing a release:

```bash
# Test Homebrew installation
brew uninstall rencore 2>/dev/null || true
brew tap stagecraft-ing/tap
brew install rencore

# Verify version
encore version

# Test basic commands
encore auth login-apikey --auth-key=test-key
encore version

# Test on different platforms (if possible)
# - macOS Intel
# - macOS Apple Silicon
# - Linux x86_64
# - Linux ARM64
```

## Release Frequency

- **Major releases**: As needed (upstream breaking changes)
- **Minor releases**: Monthly or when significant upstream updates
- **Patch releases**: As needed (bug fixes, urgent patches)
- **Nightly releases**: Daily (automated, optional)

## Communication

Announce releases via:
- [ ] GitHub release notes
- [ ] Slack/Discord announcement
- [ ] Email to users (if applicable)
- [ ] Documentation update

## Metrics to Track

For each release, track:
- Number of downloads per platform
- Installation via Homebrew vs manual
- Issues reported within 48 hours
- Time from upstream release to our release

---

**Questions?** Open an issue or discussion on GitHub.
