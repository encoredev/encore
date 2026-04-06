# Model Selection

> Determines which LLM model to use for each agent spawn.

## SCOPE

✅ THIS SKILL PRODUCES:
- A resolved `model` parameter for every `task` tool call
- Persistent model preferences in `.squad/config.json`
- Spawn acknowledgments that include the resolved model

❌ THIS SKILL DOES NOT PRODUCE:
- Code, tests, or documentation
- Model performance benchmarks
- Cost reports or billing artifacts

## Context

Squad supports 18+ models across three tiers (premium, standard, fast). The coordinator must select the right model for each agent spawn. Users can set persistent preferences that survive across sessions.

## 5-Layer Model Resolution Hierarchy

Resolution is **first-match-wins** — the highest layer with a value wins.

| Layer | Name | Source | Persistence |
|-------|------|--------|-------------|
| **0a** | Per-Agent Config | `.squad/config.json` → `agentModelOverrides.{name}` | Persistent (survives sessions) |
| **0b** | Global Config | `.squad/config.json` → `defaultModel` | Persistent (survives sessions) |
| **1** | Session Directive | User said "use X" in current session | Session-only |
| **2** | Charter Preference | Agent's `charter.md` → `## Model` section | Persistent (in charter) |
| **3** | Task-Aware Auto | Code → sonnet, docs → haiku, visual → opus | Computed per-spawn |
| **4** | Default | `claude-haiku-4.5` | Hardcoded fallback |

**Key principle:** Layer 0 (persistent config) beats everything. If the user said "always use opus" and it was saved to config.json, every agent gets opus regardless of role or task type. This is intentional — the user explicitly chose quality over cost.

## AGENT WORKFLOW

### On Session Start

1. READ `.squad/config.json`
2. CHECK for `defaultModel` field — if present, this is the Layer 0 override for all spawns
3. CHECK for `agentModelOverrides` field — if present, these are per-agent Layer 0a overrides
4. STORE both values in session context for the duration

### On Every Agent Spawn

1. CHECK Layer 0a: Is there an `agentModelOverrides.{agentName}` in config.json? → Use it.
2. CHECK Layer 0b: Is there a `defaultModel` in config.json? → Use it.
3. CHECK Layer 1: Did the user give a session directive? → Use it.
4. CHECK Layer 2: Does the agent's charter have a `## Model` section? → Use it.
5. CHECK Layer 3: Determine task type:
   - Code (implementation, tests, refactoring, bug fixes) → `claude-sonnet-4.6`
   - Prompts, agent designs → `claude-sonnet-4.6`
   - Visual/design with image analysis → `claude-opus-4.6`
   - Non-code (docs, planning, triage, changelogs) → `claude-haiku-4.5`
6. FALLBACK Layer 4: `claude-haiku-4.5`
7. INCLUDE model in spawn acknowledgment: `🔧 {Name} ({resolved_model}) — {task}`

### When User Sets a Preference

**Trigger phrases:** "always use X", "use X for everything", "switch to X", "default to X"

1. VALIDATE the model ID against the catalog (18+ models)
2. WRITE `defaultModel` to `.squad/config.json` (merge, don't overwrite)
3. ACKNOWLEDGE: `✅ Model preference saved: {model} — all future sessions will use this until changed.`

**Per-agent trigger:** "use X for {agent}"

1. VALIDATE model ID
2. WRITE to `agentModelOverrides.{agent}` in `.squad/config.json`
3. ACKNOWLEDGE: `✅ {Agent} will always use {model} — saved to config.`

### When User Clears a Preference

**Trigger phrases:** "switch back to automatic", "clear model preference", "use default models"

1. REMOVE `defaultModel` from `.squad/config.json`
2. ACKNOWLEDGE: `✅ Model preference cleared — returning to automatic selection.`

### STOP

After resolving the model and including it in the spawn template, this skill is done. Do NOT:
- Generate model comparison reports
- Run benchmarks or speed tests
- Create new config files (only modify existing `.squad/config.json`)
- Change the model after spawn (fallback chains handle runtime failures)

## Config Schema

`.squad/config.json` model-related fields:

```json
{
  "version": 1,
  "defaultModel": "claude-opus-4.6",
  "agentModelOverrides": {
    "fenster": "claude-sonnet-4.6",
    "mcmanus": "claude-haiku-4.5"
  }
}
```

- `defaultModel` — applies to ALL agents unless overridden by `agentModelOverrides`
- `agentModelOverrides` — per-agent overrides that take priority over `defaultModel`
- Both fields are optional. When absent, Layers 1-4 apply normally.

## Fallback Chains

If a model is unavailable (rate limit, plan restriction), retry within the same tier:

```
Premium:  claude-opus-4.6 → claude-opus-4.6-fast → claude-opus-4.5 → claude-sonnet-4.6
Standard: claude-sonnet-4.6 → gpt-5.4 → claude-sonnet-4.5 → gpt-5.3-codex → claude-sonnet-4
Fast:     claude-haiku-4.5 → gpt-5.1-codex-mini → gpt-4.1 → gpt-5-mini
```

**Never fall UP in tier.** A fast task won't land on a premium model via fallback.
