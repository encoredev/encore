---
name: "test-discipline"
description: "Update tests when changing APIs — no exceptions"
domain: "quality"
confidence: "high"
source: "earned (Fenster/Hockney incident, test assertion sync violations)"
---

## Context

When APIs or public interfaces change, tests must be updated in the same commit. When test assertions reference file counts or expected arrays, they must be kept in sync with disk reality. Stale tests block CI for other contributors.

## Patterns

- **API changes → test updates (same commit):** If you change a function signature, public interface, or exported API, update the corresponding tests before committing
- **Test assertions → disk reality:** When test files contain expected counts (e.g., `EXPECTED_FEATURES`, `EXPECTED_SCENARIOS`), they must match the actual files on disk
- **Add files → update assertions:** When adding docs pages, features, or any counted resource, update the test assertion array in the same commit
- **CI failures → check assertions first:** Before debugging complex failures, verify test assertion arrays match filesystem state

## Examples

✓ **Correct:**
- Changed auth API signature → updated auth.test.ts in same commit
- Added `distributed-mesh.md` to features/ → added `'distributed-mesh'` to EXPECTED_FEATURES array
- Deleted two scenario files → removed entries from EXPECTED_SCENARIOS

✗ **Incorrect:**
- Changed spawn parameters → committed without updating casting.test.ts (CI breaks for next person)
- Added `built-in-roles.md` → left EXPECTED_FEATURES at old count (PR blocked)
- Test says "expected 7 files" but disk has 25 (assertion staleness)

## Anti-Patterns

- Committing API changes without test updates ("I'll fix tests later")
- Treating test assertion arrays as static (they evolve with content)
- Assuming CI passing means coverage is correct (stale assertions can pass while being wrong)
- Leaving gaps for other agents to discover
