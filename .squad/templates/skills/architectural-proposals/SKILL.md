---
name: "architectural-proposals"
description: "How to write comprehensive architectural proposals that drive alignment before code is written"
domain: "architecture, product-direction"
confidence: "high"
source: "earned (2026-02-21 interactive shell proposal)"
tools:
  - name: "view"
    description: "Read existing codebase, prior decisions, and team context before proposing changes"
    when: "Always read .squad/decisions.md, relevant PRDs, and current architecture docs before writing proposal"
  - name: "create"
    description: "Create proposal in docs/proposals/ with structured format"
    when: "After gathering context, before any implementation work begins"
---

## Context

Proposals create alignment before code is written. Cheaper to change a doc than refactor code. Use this pattern when:
- Architecture shifts invalidate existing assumptions
- Product direction changes require new foundation
- Multiple waves/milestones will be affected by a decision
- External dependencies (Copilot CLI, SDK APIs) change

## Patterns

### Proposal Structure (docs/proposals/)

**Required sections:**
1. **Problem Statement** — Why current state is broken (specific, measurable evidence)
2. **Proposed Architecture** — Solution with technical specifics (not hand-waving)
3. **What Changes** — Impact on existing work (waves, milestones, modules)
4. **What Stays the Same** — Preserve existing functionality (no regression)
5. **Key Decisions Needed** — Explicit choices with recommendations
6. **Risks and Mitigations** — Likelihood + impact + mitigation strategy
7. **Scope** — What's in v1, what's deferred (timeline clarity)

**Optional sections:**
- Implementation Plan (high-level milestones)
- Success Criteria (measurable outcomes)
- Open Questions (unresolved items)
- Appendix (prior art, alternatives considered)

### Tone Ceiling Enforcement

**Always:**
- Cite specific evidence (user reports, performance data, failure modes)
- Justify recommendations with technical rationale
- Acknowledge trade-offs (no perfect solutions)
- Be specific about APIs, libraries, file paths

**Never:**
- Hype ("revolutionary", "game-changing")
- Hand-waving ("we'll figure it out later")
- Unsubstantiated claims ("users will love this")
- Vague timelines ("soon", "eventually")

### Wave Restructuring Pattern

When a proposal invalidates existing wave structure:
1. **Acknowledge the shift:** "This becomes Wave 0 (Foundation)"
2. **Cascade impacts:** Adjust downstream waves (Wave 1, Wave 2, Wave 3)
3. **Preserve non-blocking work:** Identify what can proceed in parallel
4. **Update dependencies:** Document new blocking relationships

**Example (Interactive Shell):**
- Wave 0 (NEW): Interactive Shell — blocks all other waves
- Wave 1 (ADJUSTED): npm Distribution — shell bundled in cli.js
- Wave 2 (DEFERRED): SquadUI — waits for shell foundation
- Wave 3 (ADJUSTED): Public Docs — now documents shell as primary interface

### Decision Framing

**Format:** "Recommendation: X (recommended) or alternatives?"

**Components:**
- Recommendation (pick one, justify)
- Alternatives (what else was considered)
- Decision rationale (why recommended option wins)
- Needs sign-off from (which agents/roles must approve)

**Example:**
```
### 1. Terminal UI Library: `ink` (recommended) or alternatives?

**Recommendation:** `ink`  
**Alternatives:** `blessed`, raw readline  
**Decision rationale:** Component model enables testable UI. Battle-tested ecosystem.

**Needs sign-off from:** Brady (product direction), Fortier (runtime performance)
```

### Risk Documentation

**Format per risk:**
- **Risk:** Specific failure mode
- **Likelihood:** Low / Medium / High (not percentages)
- **Impact:** Low / Medium / High
- **Mitigation:** Concrete actions (measurable)

**Example:**
```
### Risk 2: SDK Streaming Reliability

**Risk:** SDK streaming events might drop messages or arrive out of order.  
**Likelihood:** Low (SDK is production-grade).  
**Impact:** High — broken streaming makes shell unusable.

**Mitigation:**
- Add integration test: Send 1000-message stream, verify all deltas arrive in order
- Implement fallback: If streaming fails, fall back to polling session state
- Log all SDK events to `.squad/orchestration-log/sdk-events.jsonl` for debugging
```

## Examples

**File references from interactive shell proposal:**
- Full proposal: `docs/proposals/squad-interactive-shell.md`
- User directive: `.squad/decisions/inbox/copilot-directive-2026-02-21T202535Z.md`
- Team decisions: `.squad/decisions.md`
- Current architecture: `docs/architecture/module-map.md`, `docs/prd-23-release-readiness.md`

**Key patterns demonstrated:**
1. Read user directive first (understand the "why")
2. Survey current architecture (module map, existing waves)
3. Research SDK APIs (exploration task to validate feasibility)
4. Document problem with specific evidence (unreliable handoffs, zero visibility, UX mismatch)
5. Propose solution with technical specifics (ink components, SDK session management, spawn.ts module)
6. Restructure waves when foundation shifts (Wave 0 becomes blocker)
7. Preserve backward compatibility (squad.agent.md still works, VS Code mode unchanged)
8. Frame decisions explicitly (5 key decisions with recommendations)
9. Document risks with mitigations (5 risks, each with concrete actions)
10. Define scope (what's in v1 vs. deferred)

## Anti-Patterns

**Avoid:**
- ❌ Proposals without problem statements (solution-first thinking)
- ❌ Vague architecture ("we'll use a shell") — be specific (ink components, session registry, spawn.ts)
- ❌ Ignoring existing work — always document impact on waves/milestones
- ❌ No risk analysis — every architecture has risks, document them
- ❌ Unbounded scope — draw the v1 line explicitly
- ❌ Missing decision ownership — always say "needs sign-off from X"
- ❌ No backward compatibility plan — users don't care about your replatform
- ❌ Hand-waving timelines ("a few weeks") — be specific (2-3 weeks, 1 engineer full-time)

**Red flags in proposal reviews:**
- "Users will love this" (citation needed)
- "We'll figure out X later" (scope creep incoming)
- "This is revolutionary" (tone ceiling violation)
- No section on "What Stays the Same" (regression risk)
- No risks documented (wishful thinking)
