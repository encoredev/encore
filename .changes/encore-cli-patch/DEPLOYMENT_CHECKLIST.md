# Rencore Deployment Checklist

Use this checklist to deploy Rencore for the first time.

## Pre-Deployment

### Repository Setup
- [x] Fork encoredev/encore → stagecraft-ing/encore
- [x] Fork encoredev/homebrew-tap → stagecraft-ing/homebrew-tap
- [x] Patches created and tested
- [x] Build scripts created
- [x] GitHub Actions workflow created
- [x] Documentation written

### Verification
- [ ] All patches in `patches/` directory
- [ ] `upstream.ref` file exists
- [ ] Scripts are executable (`chmod +x scripts/*.sh`)
- [ ] Workflow file at `.github/workflows/rencore-release.yml`
- [ ] Documentation files present

## Deployment Steps

### 1. Commit Changes to encore Repository

```bash
cd /Users/bart/Dev/_stagecraft_/encore

# Check status
git status

# Add all new files
git add upstream.ref patches/ scripts/ .github/workflows/rencore-release.yml
git add internal/urlutil/ cli/cmd/encore/auth/apikey.go internal/conf/conf_custom_test.go
git add *.md .github/PULL_REQUEST_TEMPLATE.md

# Commit
git commit -m "Add Rencore release automation

- Implement patch-based customization model
- Add GitHub Actions release workflow
- Create Homebrew tap documentation
- Add comprehensive documentation
"

# Push
git push origin dev
```

- [ ] Changes committed
- [ ] Changes pushed to GitHub

### 2. Configure GitHub Secrets

Go to: https://github.com/stagecraft-ing/encore/settings/secrets/actions

**Create Personal Access Token:**
- [ ] Visit https://github.com/settings/tokens
- [ ] Create token with `repo` scope
- [ ] Copy token value

**Add to Repository Secrets:**
- [ ] Name: `HOMEBREW_TAP_TOKEN`
- [ ] Value: [Paste token]
- [ ] Click "Add secret"

### 3. Set Up Homebrew Tap

See [HOMEBREW_TAP_README.md](HOMEBREW_TAP_README.md) for complete instructions.

```bash
cd /Users/bart/Dev/_stagecraft_/

# Clone tap repository
git clone git@github.com:stagecraft-ing/homebrew-tap.git
cd homebrew-tap

# Follow the setup instructions in HOMEBREW_TAP_README.md
```

- [ ] Tap repository cloned
- [ ] Formula created
- [ ] Workflow created
- [ ] Changes committed and pushed

### 4. Create First Release

**Via GitHub UI:**
- [ ] Go to https://github.com/stagecraft-ing/encore/actions
- [ ] Click "Rencore Release"
- [ ] Click "Run workflow"
- [ ] Enter version (e.g., `v1.44.7`)
- [ ] Keep default URLs or customize
- [ ] Click "Run workflow"
- [ ] Wait for completion (~15-30 min)

### 5. Verify Release

**Check Release:**
- [ ] Visit https://github.com/stagecraft-ing/encore/releases
- [ ] Verify release exists
- [ ] Verify all 4 platform tarballs present
- [ ] Verify SHA256 files present
- [ ] Check release notes are correct

**Check Homebrew Formula:**
- [ ] Visit https://github.com/stagecraft-ing/homebrew-tap
- [ ] Verify `Formula/rencore.rb` was updated
- [ ] Check version matches release
- [ ] Check URLs are correct
- [ ] Check SHA256 values populated

### 6. Test Installation

**On macOS:**
```bash
brew tap stagecraft-ing/tap
brew install rencore
brew unlink encore 2>/dev/null || true
brew link --overwrite rencore
encore version
```

- [ ] Installation successful
- [ ] Version shows correct version
- [ ] Basic commands work

## Success Criteria

- [x] Repository structure created
- [x] Patches created and working
- [x] Build scripts functional
- [x] GitHub Actions workflow created
- [x] Documentation complete
- [ ] First release published
- [ ] Homebrew formula working
- [ ] Installation tested and verified

---

**See [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) for complete details.**
