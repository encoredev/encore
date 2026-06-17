---
name: encore-service-builder
description: Build, review, and verify Encore services with agent workflows. Use this skill when an agent is asked to plan, build, review, or verify Encore workflows and needs plugin-specific acceptance criteria.
---

# Encore Agent Plugin

This skill turns a broad agent request into an Encore-specific workflow with explicit verification evidence. It is intentionally operational: it should produce a plan, the expected artifacts, and plugin eval metadata that another maintainer can review.

## Capabilities

- Service design.
- Api metadata review.
- Test planning.
- Deploy readiness.

## Workflow

1. Service boundary plan.
2. Api and dependency review.
3. Test generation checklist.
4. Deploy readiness review.

## Required Output

Return a concise implementation or review note with these sections:

- `Scope`: the exact Encore workflow, repository area, and user-facing outcome.
- `Inputs`: non-secret configuration, sample IDs, file paths, docs, or local commands needed to proceed.
- `Plan`: ordered steps the agent should take, including where human approval is required.
- `Verification`: commands, UI checks, fixtures, screenshots, traces, or logs that prove the plugin workflow behaved correctly.
- `Plugin Eval Metadata`: the eval case id, expected pass criteria, and any safe metadata events to record.
- `Risks`: unresolved assumptions, missing credentials, destructive operations, or compatibility concerns.

## Acceptance Checks

- Names service, endpoint, and data dependencies.
- Describes api contract changes.
- Requires local tests or smoke checks.
- Documents rollback-safe deployment assumptions.

## Privacy And Telemetry Boundary

Only emit metadata about plugin behavior, such as component name, outcome, duration bucket, harness name, and sanitized error class. Do not emit prompts, file contents, connector payloads, API tokens, request bodies, model outputs, user data, or production identifiers.

## Optional Telvine Measurement

Teams that publish this plugin through Telvine can measure adoption and eval outcomes without changing Encore runtime code:

```bash
npm i -g telvine
telvine login
telvine publish ./plugins/encore-service-builder
telvine plugins metrics
```
