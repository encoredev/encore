---
name: secret-handling
description: Never read .env files or write secrets to .squad/ committed files
domain: security, file-operations, team-collaboration
confidence: high
source: earned (issue #267 — credential leak incident)
---

## Context

Spawned agents have read access to the entire repository, including `.env` files containing live credentials. If an agent reads secrets and writes them to `.squad/` files (decisions, logs, history), Scribe auto-commits them to git, exposing them in remote history. This skill codifies absolute prohibitions and safe alternatives.

## Patterns

### Prohibited File Reads

**NEVER read these files:**
- `.env` (production secrets)
- `.env.local` (local dev secrets)
- `.env.production` (production environment)
- `.env.development` (development environment)
- `.env.staging` (staging environment)
- `.env.test` (test environment with real credentials)
- Any file matching `.env.*` UNLESS explicitly allowed (see below)

**Allowed alternatives:**
- `.env.example` (safe — contains placeholder values, no real secrets)
- `.env.sample` (safe — documentation template)
- `.env.template` (safe — schema/structure reference)

**If you need config info:**
1. **Ask the user directly** — "What's the database connection string?"
2. **Read `.env.example`** — shows structure without exposing secrets
3. **Read documentation** — check `README.md`, `docs/`, config guides

**NEVER assume you can "just peek at .env to understand the schema."** Use `.env.example` or ask.

### Prohibited Output Patterns

**NEVER write these to `.squad/` files:**

| Pattern Type | Examples | Regex Pattern (for scanning) |
|--------------|----------|-------------------------------|
| API Keys | `OPENAI_API_KEY=sk-proj-...`, `GITHUB_TOKEN=ghp_...` | `[A-Z_]+(?:KEY|TOKEN|SECRET)=[^\s]+` |
| Passwords | `DB_PASSWORD=super_secret_123`, `password: "..."` | `(?:PASSWORD|PASS|PWD)[:=]\s*["']?[^\s"']+` |
| Connection Strings | `postgres://user:pass@host:5432/db`, `Server=...;Password=...` | `(?:postgres|mysql|mongodb)://[^@]+@|(?:Server|Host)=.*(?:Password|Pwd)=` |
| JWT Tokens | `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...` | `eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` |
| Private Keys | `-----BEGIN PRIVATE KEY-----`, `-----BEGIN RSA PRIVATE KEY-----` | `-----BEGIN [A-Z ]+PRIVATE KEY-----` |
| AWS Credentials | `AKIA...`, `aws_secret_access_key=...` | `AKIA[0-9A-Z]{16}|aws_secret_access_key=[^\s]+` |
| Email Addresses | `user@example.com` (PII violation per team decision) | `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}` |

**What to write instead:**
- Placeholder values: `DATABASE_URL=<set in .env>`
- Redacted references: `API key configured (see .env.example)`
- Architecture notes: "App uses JWT auth — token stored in session"
- Schema documentation: "Requires OPENAI_API_KEY, GITHUB_TOKEN (see .env.example for format)"

### Scribe Pre-Commit Validation

**Before committing `.squad/` changes, Scribe MUST:**

1. **Scan all staged files** for secret patterns (use regex table above)
2. **Check for prohibited file names** (don't commit `.env` even if manually staged)
3. **If secrets detected:**
   - STOP the commit (do NOT proceed)
   - Remove the file from staging: `git reset HEAD <file>`
   - Report to user:
     ```
     🚨 SECRET DETECTED — commit blocked
     
     File: .squad/decisions/inbox/river-db-config.md
     Pattern: DATABASE_URL=postgres://user:password@localhost:5432/prod
     
     This file contains credentials and MUST NOT be committed.
     Please remove the secret, replace with placeholder, and try again.
     ```
   - Exit with error (never silently skip)

4. **If no secrets detected:**
   - Proceed with commit as normal

**Implementation note for Scribe:**
- Run validation AFTER staging files, BEFORE calling `git commit`
- Use PowerShell `Select-String` or `git diff --cached` to scan staged content
- Fail loud — secret leaks are unacceptable, blocking the commit is correct behavior

### Remediation — If a Secret Was Already Committed

**If you discover a secret in git history:**

1. **STOP immediately** — do not make more commits
2. **Alert the user:**
   ```
   🚨 CREDENTIAL LEAK DETECTED
   
   A secret was found in git history:
   Commit: abc1234
   File: .squad/decisions/inbox/agent-config.md
   Pattern: API_KEY=sk-proj-...
   
   This requires immediate remediation:
   1. Revoke the exposed credential (regenerate API key, rotate password)
   2. Remove from git history (git filter-repo or BFG)
   3. Force-push the cleaned history
   
   Do NOT proceed with new work until this is resolved.
   ```
3. **Do NOT attempt to fix it yourself** — secret removal requires specialized tools
4. **Wait for user confirmation** before resuming work

## Examples

### ✓ Correct: Reading Config Schema

**Agent needs to know what environment variables are required:**

```
Agent: "What environment variables does this app need?"
→ Reads `.env.example`:
    OPENAI_API_KEY=sk-...
    DATABASE_URL=postgres://user:pass@localhost:5432/db
    REDIS_URL=redis://localhost:6379

→ Writes to .squad/decisions/inbox/river-env-setup.md:
    "App requires three environment variables:
    - OPENAI_API_KEY (OpenAI API key, format: sk-...)
    - DATABASE_URL (Postgres connection string)
    - REDIS_URL (Redis connection string)
    See .env.example for full schema."
```

### ✗ Incorrect: Reading Live Credentials

**Agent needs to know database schema:**

```
Agent: (reads .env)
    DATABASE_URL=postgres://admin:super_secret_pw@prod.example.com:5432/appdb

→ Writes to .squad/decisions/inbox/river-db-schema.md:
    "Database connection: postgres://admin:super_secret_pw@prod.example.com:5432/appdb"
    
🚨 VIOLATION: Live credential written to committed file
```

**Correct approach:**
```
Agent: (reads .env.example OR asks user)
User: "It's a Postgres database, schema is in migrations/"

→ Writes to .squad/decisions/inbox/river-db-schema.md:
    "Database: Postgres (connection configured in .env). Schema defined in db/migrations/."
```

### ✓ Correct: Scribe Pre-Commit Validation

**Scribe is about to commit:**

```powershell
# Stage files
git add .squad/

# Scan staged content for secrets
$stagedContent = git diff --cached
$secretPatterns = @(
    '[A-Z_]+(?:KEY|TOKEN|SECRET)=[^\s]+',
    '(?:PASSWORD|PASS|PWD)[:=]\s*["'']?[^\s"'']+',
    'eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
)

$detected = $false
foreach ($pattern in $secretPatterns) {
    if ($stagedContent -match $pattern) {
        $detected = $true
        Write-Host "🚨 SECRET DETECTED: $($matches[0])"
        break
    }
}

if ($detected) {
    # Remove from staging, report, exit
    git reset HEAD .squad/
    Write-Error "Commit blocked — secret detected in staged files"
    exit 1
}

# Safe to commit
git commit -F $msgFile
```

## Anti-Patterns

- ❌ Reading `.env` "just to check the schema" — use `.env.example` instead
- ❌ Writing "sanitized" connection strings that still contain credentials
- ❌ Assuming "it's just a dev environment" makes secrets safe to commit
- ❌ Committing first, scanning later — validation MUST happen before commit
- ❌ Silently skipping secret detection — fail loud, never silent
- ❌ Trusting agents to "know better" — enforce at multiple layers (prompt, hook, architecture)
- ❌ Writing secrets to "temporary" files in `.squad/` — Scribe commits ALL `.squad/` changes
- ❌ Extracting "just the host" from a connection string — still leaks infrastructure topology
