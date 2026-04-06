---
name: "agent-conduct"
description: "Shared hard rules enforced across all squad agents"
domain: "team-governance"
confidence: "high"
source: "reskill extraction — Product Isolation Rule and Peer Quality Check appeared in all 20 agent charters"
---

## Context

Every squad agent must follow these two hard rules. They were previously duplicated in every charter. Now they live here as a shared skill, loaded once.

## Patterns

### Product Isolation Rule (hard rule)
Tests, CI workflows, and product code must NEVER depend on specific agent names from any particular squad. "Our squad" must not impact "the squad." No hardcoded references to agent names (Flight, EECOM, FIDO, etc.) in test assertions, CI configs, or product logic. Use generic/parameterized values. If a test needs agent names, use obviously-fake test fixtures (e.g., "test-agent-1", "TestBot").

### Peer Quality Check (hard rule)
Before finishing work, verify your changes don't break existing tests. Run the test suite for files you touched. If CI has been failing, check your changes aren't contributing to the problem. When you learn from mistakes, update your history.md.

## Anti-Patterns
- Don't hardcode dev team agent names in product code or tests
- Don't skip test verification before declaring work done
- Don't ignore pre-existing CI failures that your changes may worsen
