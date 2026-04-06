---
name: history-hygiene
description: Record final outcomes to history.md, not intermediate requests or reversed decisions
domain: documentation, team-collaboration
confidence: high
source: earned (Kobayashi v0.6.0 incident, team intervention)
---

## Context

History files (.md files tracking decisions, spawns, outcomes) are read cold by future agents. Stale or incorrect entries poison decision-making downstream. The Kobayashi incident proved this: history said "Brady decided v0.6.0" when Brady had reversed that to v0.8.17. Future spawns read the wrong truth and repeated the mistake.

## Patterns

- **Record the final outcome**, not the initial request.
- **Wait for confirmation** before writing to history — don't log intermediate states.
- **If a decision reverses**, update the entry immediately — don't leave stale data.
- **One read = one truth.** A future agent should never need to cross-reference other files to understand what actually happened.

## Examples

✓ **Correct:**
- "Migration target: v0.8.17 (initially discussed as v0.6.0, corrected by Brady)"
- "Reverted to Node 18 per Brady's explicit request on 2024-01-15"

✗ **Incorrect:**
- "Brady directed v0.6.0" (when later reversed)
- Recording what was *requested* instead of what *actually happened*
- Logging entries before outcome is confirmed

## Anti-Patterns

- Writing intermediate or "for now" states to disk
- Attributing decisions without confirming final direction
- Treating history like a draft — history is the source of truth
- Assuming readers will cross-reference or verify; they won't
