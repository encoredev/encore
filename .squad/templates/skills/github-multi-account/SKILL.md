---
name: github-multi-account
description: Detect and set up account-locked gh aliases for multi-account GitHub. The AI reads this skill, detects accounts, asks the user which is personal/work, and runs the setup automatically.
confidence: high
source: https://github.com/tamirdresher/squad-skills/tree/main/plugins/github-multi-account
author: tamirdresher
---

# GitHub Multi-Account — AI-Driven Setup

## When to Activate
When the user has multiple GitHub accounts (check with `gh auth status`). If you see 2+ accounts listed, this skill applies.

## What to Do (as the AI agent)

### Step 1: Detect accounts
Run: `gh auth status`
Look for multiple accounts. Note which usernames are listed.

### Step 2: Ask the user
Ask: "I see you have multiple GitHub accounts: {list them}. Which one is your personal account and which is your work/EMU account?"

### Step 3: Run the setup automatically
Once the user confirms, do ALL of this for them:

```powershell
# 1. Define the functions
$personal = "THEIR_PERSONAL_USERNAME"
$work = "THEIR_WORK_USERNAME"

# 2. Add to PowerShell profile
$profilePath = $PROFILE.CurrentUserAllHosts
if (!(Test-Path $profilePath)) { New-Item -Path $profilePath -Force | Out-Null }
$existing = Get-Content $profilePath -Raw -ErrorAction SilentlyContinue
if ($existing -notmatch "gh-personal") {
    $block = @"

# === GitHub Multi-Account Aliases ===
function gh-personal { gh auth switch --user $personal 2>`$null | Out-Null; gh @args }
function gh-work { gh auth switch --user $work 2>`$null | Out-Null; gh @args }
Set-Alias ghp gh-personal
Set-Alias ghw gh-work
"@
    Add-Content -Path $profilePath -Value $block
}

# 3. Create CMD wrappers
$binDir = Join-Path $env:USERPROFILE ".squad\bin"
if (!(Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir -Force | Out-Null }
"@echo off`ngh auth switch --user $personal >nul 2>&1`ngh %*" | Out-File "$binDir\ghp.cmd" -Encoding ascii
"@echo off`ngh auth switch --user $work >nul 2>&1`ngh %*" | Out-File "$binDir\ghw.cmd" -Encoding ascii

# 4. Add to PATH
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notmatch [regex]::Escape($binDir)) {
    [Environment]::SetEnvironmentVariable("PATH", "$binDir;$userPath", "User")
    $env:PATH = "$binDir;$env:PATH"
}

# 5. Load in current session
function gh-personal { gh auth switch --user $personal 2>$null | Out-Null; gh @args }
function gh-work { gh auth switch --user $work 2>$null | Out-Null; gh @args }
Set-Alias ghp gh-personal
Set-Alias ghw gh-work
```

### Step 4: Verify for the user
Run these and show the results:
```powershell
ghp api user --jq '.login'   # should show personal username
ghw api user --jq '.login'   # should show work username
```

### Step 5: Tell the user
"All set! From now on use `ghp` for personal repos and `ghw` for work repos. I'll use them too."

## After Setup — Usage Rules

1. **NEVER** use bare `gh` for repo operations — always `ghp` or `ghw`
2. **NEVER** manually `gh auth switch` — the aliases handle it
3. Determine alias by repo owner:
   - Personal account repos → `ghp` / `gh-personal`
   - Work/EMU account repos → `ghw` / `gh-work`

## Repo-Specific Account Binding

This repo (`bradygaster/squad`) is bound to the **bradygaster** (personal) account.
All `gh` operations in this repo MUST use `ghp` / `gh-personal`.

## For Squad Agents
At the TOP of any script touching GitHub, define:
```powershell
function gh-personal { gh auth switch --user bradygaster 2>$null | Out-Null; gh @args }
function gh-work { gh auth switch --user bradyg_microsoft 2>$null | Out-Null; gh @args }
```
