---
name: "external-comms"
description: "PAO workflow for scanning, drafting, and presenting community responses with human review gate"
domain: "community, communication, workflow"
confidence: "low"
source: "manual (RFC #426 — PAO External Communications)"
tools:
  - name: "github-mcp-server-list_issues"
    description: "List open issues for scan candidates and lightweight triage"
    when: "Use for recent open issue scans before thread-level review"
  - name: "github-mcp-server-issue_read"
    description: "Read the full issue, comments, and labels before drafting"
    when: "Use after selecting a candidate so PAO has complete thread context"
  - name: "github-mcp-server-search_issues"
    description: "Search for candidate issues or prior squad responses"
    when: "Use when filtering by keywords, labels, or duplicate response checks"
  - name: "gh CLI"
    description: "Fallback for GitHub issue comments and discussions workflows"
    when: "Use gh issue list/comment and gh api or gh api graphql when MCP coverage is incomplete"
---

## Context

Phase 1 is **draft-only mode**.

- PAO scans issues and discussions, drafts responses with the humanizer skill, and presents a review table for human approval.
- **Human review gate is mandatory** — PAO never posts autonomously.
- Every action is logged to `.squad/comms/audit/`.
- This workflow is triggered manually only ("PAO, check community") — no automated or Ralph-triggered activation in Phase 1.

## Patterns

### 1. Scan

Find unanswered community items with GitHub MCP tools first, or `gh issue list` / `gh api` as fallback for issues and discussions.

- Include **open** issues and discussions only.
- Filter for items with **no squad team response**.
- Limit to items created in the last 7 days.
- Exclude items labeled `squad:internal` or `wontfix`.
- Include discussions **and** issues in the same sweep.
- Phase 1 scope is **issues and discussions only** — do not draft PR replies.

### Discussion Handling (Phase 1)

Discussions use the GitHub Discussions API, which differs from issues:

- **Scan:** `gh api /repos/{owner}/{repo}/discussions --jq '.[] | select(.answer_chosen_at == null)'` to find unanswered discussions
- **Categories:** Filter by Q&A and General categories only (skip Announcements, Show and Tell)
- **Answers vs comments:** In Q&A discussions, PAO drafts an "answer" (not a comment). The human marks it as accepted answer after posting.
- **Phase 1 scope:** Issues and Discussions ONLY. No PR comments.

### 2. Classify

Determine the response type before drafting.

- Welcome (new contributor)
- Troubleshooting (bug/help)
- Feature guidance (feature request/how-to)
- Redirect (wrong repo/scope)
- Acknowledgment (confirmed, no fix)
- Closing (resolved)
- Technical uncertainty (unknown cause)
- Empathetic disagreement (pushback on a decision or design)
- Information request (need more reproduction details or context)

### Template Selection Guide

| Signal in Issue/Discussion | → Response Type | Template |
|---------------------------|-----------------|----------|
| New contributor (0 prior issues) | Welcome | T1 |
| Error message, stack trace, "doesn't work" | Troubleshooting | T2 |
| "How do I...?", "Can Squad...?", "Is there a way to...?" | Feature Guidance | T3 |
| Wrong repo, out of scope for Squad | Redirect | T4 |
| Confirmed bug, no fix available yet | Acknowledgment | T5 |
| Fix shipped, PR merged that resolves issue | Closing | T6 |
| Unclear cause, needs investigation | Technical Uncertainty | T7 |
| Author disagrees with a decision or design | Empathetic Disagreement | T8 |
| Need more reproduction info or context | Information Request | T9 |

Use exactly one template as the base draft. Replace placeholders with issue-specific details, then apply the humanizer patterns. If the thread spans multiple signals, choose the highest-risk template and capture the nuance in the thread summary.

### Confidence Classification

| Confidence | Criteria | Example |
|-----------|----------|---------|
| 🟢 High | Answer exists in Squad docs or FAQ, similar question answered before, no technical ambiguity | "How do I install Squad?" |
| 🟡 Medium | Technical answer is sound but involves judgment calls, OR docs exist but don't perfectly match the question, OR tone is tricky | "Can Squad work with Azure DevOps?" (yes, but setup is nuanced) |
| 🔴 Needs Review | Technical uncertainty, policy/roadmap question, potential reputational risk, author is frustrated/angry, question about unreleased features | "When will Squad support Claude?" |

**Auto-escalation rules:**
- Any mention of competitors → 🔴
- Any mention of pricing/licensing → 🔴
- Author has >3 follow-up comments without resolution → 🔴
- Question references a closed-wontfix issue → 🔴

### 3. Draft

Use the humanizer skill for every draft.

- Complete **Thread-Read Verification** before writing.
- Read the **full thread**, including all comments, before writing.
- Select the matching template from the **Template Selection Guide** and record the template ID in the review notes.
- Treat templates as reusable drafting assets: keep the structure, replace placeholders, and only improvise when the thread truly requires it.
- Validate the draft against the humanizer anti-patterns.
- Flag long threads (`>10` comments) with `⚠️`.

### Thread-Read Verification

Before drafting, PAO MUST verify complete thread coverage:

1. **Count verification:** Compare API comment count with actually-read comments. If mismatch, abort draft.
2. **Deleted comment check:** Use `gh api` timeline to detect deleted comments. If found, flag as ⚠️ in review table.
3. **Thread summary:** Include in every draft: "Thread: {N} comments, last activity {date}, {summary of key points}"
4. **Long thread flag:** If >10 comments, add ⚠️ to review table and include condensed thread summary
5. **Evidence line in review table:** Each draft row includes "Read: {N}/{total} comments" column

### 4. Present

Show drafts for review in this exact format:

```text
📝 PAO — Community Response Drafts
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

| # | Item | Author | Type | Confidence | Read | Preview |
|---|------|--------|------|------------|------|---------|
| 1 | Issue #N | @user | Type | 🟢/🟡/🔴 | N/N | "First words..." |

Confidence: 🟢 High | 🟡 Medium | 🔴 Needs review

Full drafts below ▼
```

Each full draft must begin with the thread summary line:
`Thread: {N} comments, last activity {date}, {summary of key points}`

### 5. Human Action

Wait for explicit human direction before anything is posted.

- `pao approve 1 3` — approve drafts 1 and 3
- `pao edit 2` — edit draft 2
- `pao skip` — skip all
- `banana` — freeze all pending (safe word)

### Rollback — Bad Post Recovery

If a posted response turns out to be wrong, inappropriate, or needs correction:

1. **Delete the comment:**
   - Issues: `gh api -X DELETE /repos/{owner}/{repo}/issues/comments/{comment_id}`
   - Discussions: `gh api graphql -f query='mutation { deleteDiscussionComment(input: {id: "{node_id}"}) { comment { id } } }'`
2. **Log the deletion:** Write audit entry with action `delete`, include reason and original content
3. **Draft replacement** (if needed): PAO drafts a corrected response, goes through normal review cycle
4. **Postmortem:** If the error reveals a pattern gap, update humanizer anti-patterns or add a new test case

**Safe word — `banana`:**
- Immediately freezes all pending drafts in the review queue
- No new scans or drafts until `pao resume` is issued
- Audit entry logged with halter identity and reason

### 6. Post

After approval:

- Human posts via `gh issue comment` for issues or `gh api` for discussion answers/comments.
- PAO helps by preparing the CLI command.
- Write the audit entry after the posting action.

### 7. Audit

Log every action.

- Location: `.squad/comms/audit/{timestamp}.md`
- Required fields vary by action — see `.squad/comms/templates/audit-entry.md` Conditional Fields table
- Universal required fields: `timestamp`, `action`
- All other fields are conditional on the action type

## Examples

These are reusable templates. Keep the structure, replace placeholders, and adjust only where the thread requires it.

### Example scan command

```bash
gh issue list --state open --json number,title,author,labels,comments --limit 20
```

### Example review table

```text
📝 PAO — Community Response Drafts
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

| # | Item | Author | Type | Confidence | Read | Preview |
|---|------|--------|------|------------|------|---------|
| 1 | Issue #426 | @newdev | Welcome | 🟢 | 1/1 | "Hey @newdev! Welcome to Squad..." |
| 2 | Discussion #18 | @builder | Feature guidance | 🟡 | 4/4 | "Great question! Today the CLI..." |
| 3 | Issue #431 ⚠️ | @debugger | Technical uncertainty | 🔴 | 12/12 | "Interesting find, @debugger..." |

Confidence: 🟢 High | 🟡 Medium | 🔴 Needs review

Full drafts below ▼
```

### Example audit entry (post action)

```markdown
---
timestamp: "2026-03-16T21:30:00Z"
action: "post"
item_number: 426
draft_id: 1
reviewer: "@bradygaster"
---

## Context (draft, approve, edit, skip, post, delete actions)
- Thread depth: 3
- Response type: welcome
- Confidence: 🟢
- Long thread flag: false

## Draft Content (draft, edit, post actions)
Thread: 3 comments, last activity 2026-03-16, reporter hit a preview-build regression after install.

Hey @newdev! Welcome to Squad 👋 Thanks for opening this.
We reproduced the issue in preview builds and we're checking the regression point now.
Let us know if you can share the command you ran right before the failure.

## Post Result (post, delete actions)
https://github.com/bradygaster/squad/issues/426#issuecomment-123456
```

### T1 — Welcome

```text
Hey {author}! Welcome to Squad 👋 Thanks for opening this.
{specific acknowledgment or first answer}
Let us know if you have questions — happy to help!
```

### T2 — Troubleshooting

```text
Thanks for the detailed report, {author}!
Here's what we think is happening: {explanation}
{steps or workaround}
Let us know if that helps, or if you're seeing something different.
```

### T3 — Feature Guidance

```text
Great question! {context on current state}
{guidance or workaround}
We've noted this as a potential improvement — {tracking info if applicable}.
```

### T4 — Redirect

```text
Thanks for reaching out! This one is actually better suited for {correct location}.
{brief explanation of why}
Feel free to open it there — they'll be able to help!
```

### T5 — Acknowledgment

```text
Good catch, {author}. We've confirmed this is a real issue.
{what we know so far}
We'll update this thread when we have a fix. Thanks for flagging it!
```

### T6 — Closing

```text
This should be resolved in {version/PR}! 🎉
{brief summary of what changed}
Thanks for reporting this, {author} — it made Squad better.
```

### T7 — Technical Uncertainty

```text
Interesting find, {author}. We're not 100% sure what's causing this yet.
Here's what we've ruled out: {list}
We'd love more context if you have it — {specific ask}.
We'll dig deeper and update this thread.
```

### T8 — Empathetic Disagreement

```text
We hear you, {author}. That's a fair concern.

The current design choice was driven by {reason}. We know it's not ideal for every use case.

{what alternatives exist or what trade-off was made}

If you have ideas for how to make this work better for your scenario, we'd love to hear them — open a discussion or drop your thoughts here!
```

### T9 — Information Request

```text
Thanks for reporting this, {author}!

To help us dig into this, could you share:
- {specific ask 1}
- {specific ask 2}
- {specific ask 3, if applicable}

That context will help us narrow down what's happening. Appreciate it!
```

## Anti-Patterns

- ❌ Posting without human review (NEVER — this is the cardinal rule)
- ❌ Drafting without reading full thread (context is everything)
- ❌ Ignoring confidence flags (🔴 items need Flight/human review)
- ❌ Scanning closed issues (only open items)
- ❌ Responding to issues labeled `squad:internal` or `wontfix`
- ❌ Skipping audit logging (every action must be recorded)
- ❌ Drafting for issues where a squad member already responded (avoid duplicates)
- ❌ Drafting pull request responses in Phase 1 (issues/discussions only)
- ❌ Treating templates like loose examples instead of reusable drafting assets
- ❌ Asking for more info without specific requests
