---
name: "docs-standards"
description: "Microsoft Style Guide + Squad-specific documentation patterns"
domain: "documentation"
confidence: "high"
source: "earned (PAO charter, multiple doc PR reviews)"
---

## Context

Squad documentation follows the Microsoft Style Guide with Squad-specific conventions. Consistency across docs builds trust and improves discoverability.

## Patterns

### Microsoft Style Guide Rules
- **Sentence-case headings:** "Getting started" not "Getting Started"
- **Active voice:** "Run the command" not "The command should be run"
- **Second person:** "You can configure..." not "Users can configure..."
- **Present tense:** "The system routes..." not "The system will route..."
- **No ampersands in prose:** "and" not "&" (except in code, brand names, or UI elements)

### Squad Formatting Patterns
- **Scannability first:** Paragraphs for narrative (3-4 sentences max), bullets for scannable lists, tables for structured data
- **"Try this" prompts at top:** Start feature/scenario pages with practical prompts users can copy
- **Experimental warnings:** Features in preview get callout at top
- **Cross-references at bottom:** Related pages linked after main content

### Structure
- **Title (H1)** → **Warning/callout** → **Try this code** → **Overview** → **HR** → **Content (H2 sections)**

### Test Sync Rule
- **Always update test assertions:** When adding docs pages to `features/`, `scenarios/`, `guides/`, update corresponding `EXPECTED_*` arrays in `test/docs-build.test.ts` in the same commit

## Examples

✓ **Correct:**
```markdown
# Getting started with Squad

> ⚠️ **Experimental:** This feature is in preview.

Try this:
\`\`\`bash
squad init
\`\`\`

Squad helps you build AI teams...

---

## Install Squad

Run the following command...
```

✗ **Incorrect:**
```markdown
# Getting Started With Squad  // Title case

Squad is a tool which will help users... // Third person, future tense

You can install Squad with npm & configure it... // Ampersand in prose
```

## Anti-Patterns

- Title-casing headings because "it looks nicer"
- Writing in passive voice or third person
- Long paragraphs of dense text (breaks scannability)
- Adding doc pages without updating test assertions
- Using ampersands outside code blocks
