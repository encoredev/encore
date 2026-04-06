---
name: "gh-auth-isolation"
description: "Safely manage multiple GitHub identities (EMU + personal) in agent workflows"
domain: "security, github-integration, authentication, multi-account"
confidence: "high"
source: "earned (production usage across 50+ sessions with EMU corp + personal GitHub accounts)"
tools:
  - name: "gh"
    description: "GitHub CLI for authenticated operations"
    when: "When accessing GitHub resources requiring authentication"
---

## Context

Many developers use GitHub through an Enterprise Managed User (EMU) account at work while maintaining a personal GitHub account for open-source contributions. AI agents spawned by Squad inherit the shell's default `gh` authentication — which is usually the EMU account. This causes failures when agents try to push to personal repos, create PRs on forks, or interact with resources outside the enterprise org.

This skill teaches agents how to detect the active identity, switch contexts safely, and avoid mixing credentials across operations.

## Patterns

### Detect Current Identity

Before any GitHub operation, check which account is active:

```bash
gh auth status
```

Look for:
- `Logged in to github.com as USERNAME` — the active account
- `Token scopes: ...` — what permissions are available
- Multiple accounts will show separate entries

### Extract a Specific Account's Token

When you need to operate as a specific user (not the default):

```bash
# Get the personal account token (by username)
gh auth token --user personaluser

# Get the EMU account token
gh auth token --user corpalias_enterprise
```

**Use case:** Push to a personal fork while the default `gh` auth is the EMU account.

### Push to Personal Repos from EMU Shell

The most common scenario: your shell defaults to the EMU account, but you need to push to a personal GitHub repo.

```bash
# 1. Extract the personal token
$token = gh auth token --user personaluser

# 2. Push using token-authenticated HTTPS
git push https://personaluser:$token@github.com/personaluser/repo.git branch-name
```

**Why this works:** `gh auth token --user` reads from `gh`'s credential store without switching the active account. The token is used inline for a single operation and never persisted.

### Create PRs on Personal Forks

When the default `gh` context is EMU but you need to create a PR from a personal fork:

```bash
# Option 1: Use --repo flag (works if token has access)
gh pr create --repo upstream/repo --head personaluser:branch --title "..." --body "..."

# Option 2: Temporarily set GH_TOKEN for one command
$env:GH_TOKEN = $(gh auth token --user personaluser)
gh pr create --repo upstream/repo --head personaluser:branch --title "..."
Remove-Item Env:\GH_TOKEN
```

### Config Directory Isolation (Advanced)

For complete isolation between accounts, use separate `gh` config directories:

```bash
# Personal account operations
$env:GH_CONFIG_DIR = "$HOME/.config/gh-public"
gh auth login  # Login with personal account (one-time setup)
gh repo clone personaluser/repo

# EMU account operations (default)
Remove-Item Env:\GH_CONFIG_DIR
gh auth status  # Back to EMU account
```

**Setup (one-time):**
```bash
# Create isolated config for personal account
mkdir ~/.config/gh-public
$env:GH_CONFIG_DIR = "$HOME/.config/gh-public"
gh auth login --web --git-protocol https
```

### Shell Aliases for Quick Switching

Add to your shell profile for convenience:

```powershell
# PowerShell profile
function ghp { $env:GH_CONFIG_DIR = "$HOME/.config/gh-public"; gh @args; Remove-Item Env:\GH_CONFIG_DIR }
function ghe { gh @args }  # Default EMU

# Usage:
# ghp repo clone personaluser/repo   # Uses personal account
# ghe issue list                       # Uses EMU account
```

```bash
# Bash/Zsh profile
alias ghp='GH_CONFIG_DIR=~/.config/gh-public gh'
alias ghe='gh'

# Usage:
# ghp repo clone personaluser/repo
# ghe issue list
```

## Examples

### ✓ Correct: Agent pushes blog post to personal GitHub Pages

```powershell
# Agent needs to push to personaluser.github.io (personal repo)
# Default gh auth is corpalias_enterprise (EMU)

$token = gh auth token --user personaluser
git remote set-url origin https://personaluser:$token@github.com/personaluser/personaluser.github.io.git
git push origin main

# Clean up — don't leave token in remote URL
git remote set-url origin https://github.com/personaluser/personaluser.github.io.git
```

### ✓ Correct: Agent creates a PR from personal fork to upstream

```powershell
# Fork: personaluser/squad, Upstream: bradygaster/squad
# Agent is on branch contrib/fix-docs in the fork clone

git push origin contrib/fix-docs  # Pushes to fork (may need token auth)

# Create PR targeting upstream
gh pr create --repo bradygaster/squad --head personaluser:contrib/fix-docs `
  --title "docs: fix installation guide" `
  --body "Fixes #123"
```

### ✗ Incorrect: Blindly pushing with wrong account

```bash
# BAD: Agent assumes default gh auth works for personal repos
git push origin main
# ERROR: Permission denied — EMU account has no access to personal repo

# BAD: Hardcoding tokens in scripts
git push https://personaluser:ghp_xxxxxxxxxxxx@github.com/personaluser/repo.git main
# SECURITY RISK: Token exposed in command history and process list
```

### ✓ Correct: Check before you push

```bash
# Always verify which account has access before operations
gh auth status
# If wrong account, use token extraction:
$token = gh auth token --user personaluser
git push https://personaluser:$token@github.com/personaluser/repo.git main
```

## Anti-Patterns

- ❌ **Hardcoding tokens** in scripts, environment variables, or committed files. Use `gh auth token --user` to extract at runtime.
- ❌ **Assuming the default `gh` auth works** for all repos. EMU accounts can't access personal repos and vice versa.
- ❌ **Switching `gh auth login`** globally mid-session. This changes the default for ALL processes and can break parallel agents.
- ❌ **Storing personal tokens in `.env`** or `.squad/` files. These get committed by Scribe. Use `gh`'s credential store.
- ❌ **Ignoring token cleanup** after inline HTTPS pushes. Always reset the remote URL to avoid persisting tokens.
- ❌ **Using `gh auth switch`** in multi-agent sessions. One agent switching affects all others sharing the shell.
- ❌ **Mixing EMU and personal operations** in the same git clone. Use separate clones or explicit remote URLs per operation.
