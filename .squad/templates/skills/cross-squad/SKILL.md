---
name: "cross-squad"
description: "Coordinating work across multiple Squad instances"
domain: "orchestration"
confidence: "medium"
source: "manual"
tools:
  - name: "squad-discover"
    description: "List known squads and their capabilities"
    when: "When you need to find which squad can handle a task"
  - name: "squad-delegate"
    description: "Create work in another squad's repository"
    when: "When a task belongs to another squad's domain"
---

## Context
When an organization runs multiple Squad instances (e.g., platform-squad, frontend-squad, data-squad), those squads need to discover each other, share context, and hand off work across repository boundaries. This skill teaches agents how to coordinate across squads without creating tight coupling.

Cross-squad orchestration applies when:
- A task requires capabilities owned by another squad
- An architectural decision affects multiple squads
- A feature spans multiple repositories with different squads
- A squad needs to request infrastructure, tooling, or support from another squad

## Patterns

### Discovery via Manifest
Each squad publishes a `.squad/manifest.json` declaring its name, capabilities, and contact information. Squads discover each other through:
1. **Well-known paths**: Check `.squad/manifest.json` in known org repos
2. **Upstream config**: Squads already listed in `.squad/upstream.json` are checked for manifests
3. **Explicit registry**: A central `squad-registry.json` can list all squads in an org

```json
{
  "name": "platform-squad",
  "version": "1.0.0",
  "description": "Platform infrastructure team",
  "capabilities": ["kubernetes", "helm", "monitoring", "ci-cd"],
  "contact": {
    "repo": "org/platform",
    "labels": ["squad:platform"]
  },
  "accepts": ["issues", "prs"],
  "skills": ["helm-developer", "operator-developer", "pipeline-engineer"]
}
```

### Context Sharing
When delegating work, share only what the target squad needs:
- **Capability list**: What this squad can do (from manifest)
- **Relevant decisions**: Only decisions that affect the target squad
- **Handoff context**: A concise description of why this work is being delegated

Do NOT share:
- Internal team state (casting history, session logs)
- Full decision archives (send only relevant excerpts)
- Authentication credentials or secrets

### Work Handoff Protocol
1. **Check manifest**: Verify the target squad accepts the work type (issues, PRs)
2. **Create issue**: Use `gh issue create` in the target repo with:
   - Title: `[cross-squad] <description>`
   - Label: `squad:cross-squad` (or the squad's configured label)
   - Body: Context, acceptance criteria, and link back to originating issue
3. **Track**: Record the cross-squad issue URL in the originating squad's orchestration log
4. **Poll**: Periodically check if the delegated issue is closed/completed

### Feedback Loop
Track delegated work completion:
- Poll target issue status via `gh issue view`
- Update originating issue with status changes
- Close the feedback loop when delegated work merges

## Examples

### Discovering squads
```bash
# List all squads discoverable from upstreams and known repos
squad discover

# Output:
#   platform-squad  →  org/platform  (kubernetes, helm, monitoring)
#   frontend-squad  →  org/frontend  (react, nextjs, storybook)
#   data-squad      →  org/data      (spark, airflow, dbt)
```

### Delegating work
```bash
# Delegate a task to the platform squad
squad delegate platform-squad "Add Prometheus metrics endpoint for the auth service"

# Creates issue in org/platform with cross-squad label and context
```

### Manifest in squad.config.ts
```typescript
export default defineSquad({
  manifest: {
    name: 'platform-squad',
    capabilities: ['kubernetes', 'helm'],
    contact: { repo: 'org/platform', labels: ['squad:platform'] },
    accepts: ['issues', 'prs'],
    skills: ['helm-developer', 'operator-developer'],
  },
});
```

## Anti-Patterns
- **Direct file writes across repos** — Never modify another squad's `.squad/` directory. Use issues and PRs as the communication protocol.
- **Tight coupling** — Don't depend on another squad's internal structure. Use the manifest as the public API contract.
- **Unbounded delegation** — Always include acceptance criteria and a timeout. Don't create open-ended requests.
- **Skipping discovery** — Don't hardcode squad locations. Use manifests and the discovery protocol.
- **Sharing secrets** — Never include credentials, tokens, or internal URLs in cross-squad issues.
- **Circular delegation** — Track delegation chains. If squad A delegates to B which delegates back to A, something is wrong.
