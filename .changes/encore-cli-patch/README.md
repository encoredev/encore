# Rencore - Encore CLI Customization

This directory contains the complete documentation for the Rencore project - a custom build of the Encore CLI configured for self-hosted platform deployments.

## Documentation Structure

### Core Documentation (Root Level)
Located in the repository root:

1. **[RENCORE.md](../../RENCORE.md)** - Main project documentation
   - Architecture overview
   - Patches applied
   - Configuration reference
   - Development guide

2. **[QUICKSTART.md](../../QUICKSTART.md)** - 5-minute quick start guide
   - Installation instructions
   - Authentication setup
   - First app creation
   - Common commands

3. **[DEPLOYMENT_CHECKLIST.md](../../DEPLOYMENT_CHECKLIST.md)** - Step-by-step deployment guide
   - Pre-deployment verification
   - GitHub setup
   - Homebrew tap configuration
   - Release creation

4. **[RELEASES.md](../../RELEASES.md)** - Release process documentation
   - Release types (stable, beta, nightly)
   - Creating releases
   - Version management
   - Rollback procedures

5. **[HOMEBREW_TAP_README.md](../../HOMEBREW_TAP_README.md)** - Homebrew tap setup
   - Formula structure
   - Auto-update workflow
   - Testing procedures

6. **[IMPLEMENTATION_SUMMARY.md](../../IMPLEMENTATION_SUMMARY.md)** - Complete implementation overview
   - What was built
   - Files created/modified
   - Deployment steps
   - Success metrics

### Historical Documentation (This Directory)

This directory contains historical planning and implementation documents:

- **[task.md](task.md)** - Original task breakdown and completion tracking
- **[implementation_plan.md](implementation_plan.md)** - Original implementation plan (Strategy 3)
- **[walkthrough.md](walkthrough.md)** - Original feature walkthrough
- **[encore-customization-worklog.md](encore-customization-worklog.md)** - Original worklog and maintenance plan
- **[CURRENT_STATUS.md](CURRENT_STATUS.md)** - ⭐ Current implementation status (updated)
- **[MIGRATION_GUIDE.md](MIGRATION_GUIDE.md)** - ⭐ Guide for transitioning from old to new approach (new)

### Reference Documentation (Root Level)

Background and research documents:

- **[Encore CLI_ Code Changes for Self-Hosted Platform (1).md](../../Encore%20CLI_%20Code%20Changes%20for%20Self-Hosted%20Platform%20(1).md)**
  - Original feasibility analysis
  - Code modification strategies
  - Build instructions

- **[Encore CLI Local HTTPS Flag Feasibility & Implementation Plan.md](../../Encore%20CLI%20Local%20HTTPS%20Flag%20Feasibility%20&%20Implementation%20Plan.md)**
  - HTTPS implementation plan (not yet implemented)
  - Certificate management approach
  - Future enhancement

## Quick Navigation

### For Users

**Getting Started:**
1. Read [QUICKSTART.md](../../QUICKSTART.md) for installation
2. Follow [DEPLOYMENT_CHECKLIST.md](../../DEPLOYMENT_CHECKLIST.md) for deployment

**Reference:**
- [RENCORE.md](../../RENCORE.md) - Full documentation
- [RELEASES.md](../../RELEASES.md) - Release process

### For Developers

**Understanding the Code:**
1. Read [CURRENT_STATUS.md](CURRENT_STATUS.md) - Current implementation
2. Review patches in `../../patches/`
3. See [RENCORE.md](../../RENCORE.md) - Development section

**Making Changes:**
1. Review [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) - Approach evolution
2. Follow [RELEASES.md](../../RELEASES.md) - Release process
3. Update patches in `../../patches/`

### For Maintainers

**Upstream Sync:**
1. Review [CURRENT_STATUS.md](CURRENT_STATUS.md) - Maintenance section
2. Update `../../upstream.ref`
3. Regenerate patches if needed

**Creating Releases:**
1. Follow [DEPLOYMENT_CHECKLIST.md](../../DEPLOYMENT_CHECKLIST.md)
2. Use GitHub Actions workflow
3. Verify Homebrew formula update

## Implementation Approach Evolution

### Original Approach (Historical)
- **Strategy 3**: Direct code modifications
- **Method**: In-place file edits in vendor/encore
- **Challenges**: Merge conflicts on upstream sync

### Current Approach (Implemented)
- **Strategy B**: Patch bundle model
- **Method**: Automated patch application from clean upstream
- **Benefits**: Clean separation, easy upstream sync

See [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) for details on the transition.

## Key Achievements

✅ **Self-Hosted Platform Support**
- Custom platform API URLs
- Custom web dashboard URLs
- Custom documentation URLs
- Environment variable overrides

✅ **API Key Authentication**
- `encore auth login-apikey` command
- Stdin support for CI/CD
- Seamless integration

✅ **Automated Release Process**
- Multi-platform builds
- GitHub Actions workflow
- Homebrew formula auto-update

✅ **Clean Maintenance**
- Patch-based approach
- Easy upstream sync
- No merge conflicts

## File Organization

```
encore/
├── .changes/
│   └── encore-cli-patch/
│       ├── README.md                          # This file
│       ├── CURRENT_STATUS.md                  # Current implementation status
│       ├── MIGRATION_GUIDE.md                 # Approach evolution guide
│       ├── QUICKSTART.md                      # Quick start guide
│       ├── DEPLOYMENT_CHECKLIST.md            # Deployment guide
│       ├── RELEASES.md                        # Release process
│       ├── HOMEBREW_TAP_README.md             # Homebrew setup
│       ├── IMPLEMENTATION_SUMMARY.md          # Implementation overview
│       ├── task.md                            # Historical task list
│       ├── implementation_plan.md             # Historical implementation plan
│       ├── walkthrough.md                     # Historical walkthrough
│       └── encore-customization-worklog.md    # Historical worklog
├── patches/                                   # Current patches
│   ├── 0001-add-urlutil-package.patch
│   ├── 0002-self-hosted-platform-urls.patch
│   ├── 0003-add-api-key-authentication.patch
│   └── 0004-update-docs-urls.patch
├── scripts/                                   # Automation scripts
│   ├── apply_patches.sh
│   └── build.sh
├── upstream.ref                               # Pinned upstream commit
└── RENCORE.md                                 # Main documentation
```

## Getting Help

- **Issues**: https://github.com/stagecraft-ing/encore/issues
- **Discussions**: https://github.com/stagecraft-ing/encore/discussions
- **Documentation**: Start with [QUICKSTART.md](./QUICKSTART.md)

## Contributing

See root-level documentation:
- [RENCORE.md](../../RENCORE.md) - Development section
- [RELEASES.md](../../RELEASES.md) - Release process
- Pull requests welcome!

---

**Last Updated**: February 4, 2026
**Status**: Production Ready ✅
