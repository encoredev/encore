---
name: "reskill"
description: "Team-wide charter and history optimization through skill extraction"
domain: "team-optimization"
confidence: "high"
source: "manual — Brady directive to reduce per-agent context overhead"
---

## Context

When the coordinator hears "team, reskill" (or similar: "optimize context", "slim down charters"), trigger a team-wide optimization pass. The goal: reduce per-agent context consumption by extracting shared patterns from charters and histories into reusable skills.

This is a periodic maintenance activity. Run whenever charter/history bloat is suspected.

## Process

### Step 1: Audit
Read all agent charters and histories. Measure byte sizes. Identify:

- **Boilerplate** — sections repeated across ≥3 charters with <10% variation (collaboration, model, boundaries template)
- **Shared knowledge** — domain knowledge duplicated in 2+ charters (incident postmortems, technical patterns)
- **Mature learnings** — history entries appearing 3+ times across agents that should be promoted to skills

### Step 2: Extract
For each identified pattern:
1. Create or update a skill at `.squad/skills/{skill-name}/SKILL.md`
2. Follow the skill template format (frontmatter + Context + Patterns + Examples + Anti-Patterns)
3. Set confidence: low (first observation), medium (2+ agents), high (team-wide)

### Step 3: Trim
**Charters** — target ≤1.5KB per agent:
- Remove Collaboration section entirely (spawn prompt + agent-collaboration skill covers it)
- Remove Voice section (tagline blockquote at top of charter already captures it)
- Trim Model section to single line: `Preferred: {model}`
- Remove "When I'm unsure" boilerplate from Boundaries
- Remove domain knowledge now covered by a skill — add skill reference comment if helpful
- Keep: Identity, What I Own, unique How I Work patterns, Boundaries (domain list only)

**Histories** — target ≤8KB per agent:
- Apply history-hygiene skill to any history >12KB
- Promote recurring patterns (3+ occurrences across agents) to skills
- Summarize old entries into `## Core Context` section
- Remove session-specific metadata (dates, branch names, requester names)

### Step 4: Report
Output a savings table:

| Agent | Charter Before | Charter After | History Before | History After | Saved |
|-------|---------------|---------------|----------------|---------------|-------|

Include totals and percentage reduction.

## Patterns

### Minimal Charter Template (target format after reskill)

```
# {Name} — {Role}

> {Tagline — one sentence capturing voice and philosophy}

## Identity
- **Name:** {Name}
- **Role:** {Role}
- **Expertise:** {comma-separated list}

## What I Own
- {bullet list of owned artifacts/domains}

## How I Work
- {unique patterns and principles — NOT boilerplate}

## Boundaries
**I handle:** {domain list}
**I don't handle:** {explicit exclusions}

## Model
Preferred: {model}
```

### Skill Extraction Threshold
- **1 charter** → leave in charter (unique to that agent)
- **2 charters** → consider extracting if >500 bytes of overlap
- **3+ charters** → always extract to a shared skill

## Anti-Patterns
- Don't delete unique per-agent identity or domain-specific knowledge
- Don't create skills for content only one agent uses
- Don't merge unrelated patterns into a single mega-skill
- Don't remove Model preference line (coordinator needs it for model selection)
- Don't touch `.squad/decisions.md` during reskill
- Don't remove the tagline blockquote — it's the charter's soul in one line
