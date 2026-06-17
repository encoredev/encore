# Encore Agent Plugin

This plugin packages a focused agent workflow for Encore maintainers and users. It is designed to be useful in Codex, Claude Code, Claude Cowork, Copilot-style coworkers, and other `SKILL.md`-compatible harnesses.

The plugin does not add a runtime dependency to Encore. It gives agents a precise operating procedure, expected outputs, and plugin evals so maintainers can decide whether agent-produced work is good enough to accept.

## What It Includes

- Codex and Claude plugin manifests.
- An Encore-specific skill at `skills/encore-service-builder/SKILL.md`.
- Plugin eval cases in `evals/encore-service-builder/cases.jsonl`.
- Privacy-safe measurement guidance for teams that want production plugin metrics.

## Primary Workflows

- Service boundary plan.
- Api and dependency review.
- Test generation checklist.
- Deploy readiness review.

## Eval Cases

- `service-plan`: Plan a new Encore service endpoint for creating billing events.
- `api-review`: Review an Encore API change for breaking clients.
- `deploy-check`: Create deployment readiness checks for an Encore service change.

## Install In An Agent Harness

Use this plugin directory directly from the repository when your harness supports local or Git-backed plugin sources. The plugin root is:

```text
plugins/encore-service-builder
```

For Telvine-backed distribution and metrics:

```bash
npm i -g telvine
telvine login
telvine publish ./plugins/encore-service-builder
telvine plugins metrics
```

## Telemetry Boundary

The plugin should only record metadata about plugin execution and eval outcomes. Do not record prompts, source files, request bodies, connector payloads, credentials, model outputs, or production user data.
