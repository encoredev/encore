---
name: "session-recovery"
description: "Find and resume interrupted Copilot CLI sessions using session_store queries"
domain: "workflow-recovery"
confidence: "high"
source: "earned"
tools:
  - name: "sql"
    description: "Query session_store database for past session history"
    when: "Always — session_store is the source of truth for session history"
---

## Context

Squad agents run in Copilot CLI sessions that can be interrupted — terminal crashes, network drops, machine restarts, or accidental window closes. When this happens, in-progress work may be left in a partially-completed state: branches with uncommitted changes, issues marked in-progress with no active agent, or checkpoints that were never finalized.

Copilot CLI stores session history in a SQLite database called `session_store` (read-only, accessed via the `sql` tool with `database: "session_store"`). This skill teaches agents how to query that store to detect interrupted sessions and resume work.

## Patterns

### 1. Find Recent Sessions

Query the `sessions` table filtered by time window. Include the last checkpoint to understand where the session stopped:

```sql
SELECT
  s.id,
  s.summary,
  s.cwd,
  s.branch,
  s.updated_at,
  (SELECT title FROM checkpoints
   WHERE session_id = s.id
   ORDER BY checkpoint_number DESC LIMIT 1) AS last_checkpoint
FROM sessions s
WHERE s.updated_at >= datetime('now', '-24 hours')
ORDER BY s.updated_at DESC;
```

### 2. Filter Out Automated Sessions

Automated agents (monitors, keep-alive, heartbeat) create high-volume sessions that obscure human-initiated work. Exclude them:

```sql
SELECT s.id, s.summary, s.cwd, s.updated_at,
  (SELECT title FROM checkpoints
   WHERE session_id = s.id
   ORDER BY checkpoint_number DESC LIMIT 1) AS last_checkpoint
FROM sessions s
WHERE s.updated_at >= datetime('now', '-24 hours')
  AND s.id NOT IN (
    SELECT DISTINCT t.session_id FROM turns t
    WHERE t.turn_index = 0
      AND (LOWER(t.user_message) LIKE '%keep-alive%'
           OR LOWER(t.user_message) LIKE '%heartbeat%')
  )
ORDER BY s.updated_at DESC;
```

### 3. Search by Topic (FTS5)

Use the `search_index` FTS5 table for keyword search. Expand queries with synonyms since this is keyword-based, not semantic:

```sql
SELECT DISTINCT s.id, s.summary, s.cwd, s.updated_at
FROM search_index si
JOIN sessions s ON si.session_id = s.id
WHERE search_index MATCH 'auth OR login OR token OR JWT'
  AND s.updated_at >= datetime('now', '-48 hours')
ORDER BY s.updated_at DESC
LIMIT 10;
```

### 4. Search by Working Directory

```sql
SELECT s.id, s.summary, s.updated_at,
  (SELECT title FROM checkpoints
   WHERE session_id = s.id
   ORDER BY checkpoint_number DESC LIMIT 1) AS last_checkpoint
FROM sessions s
WHERE s.cwd LIKE '%my-project%'
  AND s.updated_at >= datetime('now', '-48 hours')
ORDER BY s.updated_at DESC;
```

### 5. Get Full Session Context Before Resuming

Before resuming, inspect what the session was doing:

```sql
-- Conversation turns
SELECT turn_index, substr(user_message, 1, 200) AS ask, timestamp
FROM turns WHERE session_id = 'SESSION_ID' ORDER BY turn_index;

-- Checkpoint progress
SELECT checkpoint_number, title, overview
FROM checkpoints WHERE session_id = 'SESSION_ID' ORDER BY checkpoint_number;

-- Files touched
SELECT file_path, tool_name
FROM session_files WHERE session_id = 'SESSION_ID';

-- Linked PRs/issues/commits
SELECT ref_type, ref_value
FROM session_refs WHERE session_id = 'SESSION_ID';
```

### 6. Detect Orphaned Issue Work

Find sessions that were working on issues but may not have completed:

```sql
SELECT DISTINCT s.id, s.branch, s.summary, s.updated_at,
  sr.ref_type, sr.ref_value
FROM sessions s
JOIN session_refs sr ON s.id = sr.session_id
WHERE sr.ref_type = 'issue'
  AND s.updated_at >= datetime('now', '-48 hours')
ORDER BY s.updated_at DESC;
```

Cross-reference with `gh issue list --label "status:in-progress"` to find issues that are marked in-progress but have no active session.

### 7. Resume a Session

Once you have the session ID:

```bash
# Resume directly
copilot --resume SESSION_ID
```

## Examples

**Recovering from a crash during PR creation:**
1. Query recent sessions filtered by branch name
2. Find the session that was working on the PR
3. Check its last checkpoint — was the code committed? Was the PR created?
4. Resume or manually complete the remaining steps

**Finding yesterday's work on a feature:**
1. Use FTS5 search with feature keywords
2. Filter to the relevant working directory
3. Review checkpoint progress to see how far the session got
4. Resume if work remains, or start fresh with the context

## Anti-Patterns

- ❌ Searching by partial session IDs — always use full UUIDs
- ❌ Resuming sessions that completed successfully — they have no pending work
- ❌ Using `MATCH` with special characters without escaping — wrap paths in double quotes
- ❌ Skipping the automated-session filter — high-volume automated sessions will flood results
- ❌ Assuming FTS5 is semantic search — it's keyword-based; always expand queries with synonyms
- ❌ Ignoring checkpoint data — checkpoints show exactly where the session stopped
