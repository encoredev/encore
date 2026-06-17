---
seotitle: The Encore Development Workflow — A tight iteration loop for humans and AI agents
seodesc: Encore gives you the same infrastructure model from local development, through per-PR preview environments, to production on AWS or GCP. The fast feedback loop makes it especially well-suited to AI coding agents.
title: Development Workflow
subtitle: A tight iteration loop from your laptop to AWS or GCP, designed for fast feedback with humans and AI agents
lang: platform
---

AI coding agents are most effective when they can validate their own changes and iterate quickly. The harder it is to actually run a change end-to-end, the more an agent (or a human) has to guess instead of verify.

Traditional Terraform-based workflows are production-centric. The infrastructure model lives in a separate codebase, applies only against cloud accounts, and is awkward or impossible to run on a laptop. Teams compensate by building bespoke local approximations (Docker Compose files, hand-rolled mocks, staging-only testing), but those approximations drift from production. The result is a slow loop where most changes can only really be validated after a deploy.

Encore is built around a different assumption: the **same infrastructure model** should drive local development, per-PR preview environments, and production. Because that model is your code, it can run anywhere, which makes the whole iteration loop faster for everyone, and dramatically more effective for AI agents.

## Develop locally as if the infrastructure is already set up

You declare infrastructure (SQL databases, Pub/Sub, object storage, caches, cron jobs, secrets) as objects in your application code using the open source Encore [TypeScript](/docs/ts) or [Go](/docs/go) SDK.

`encore run` boots the whole system: real Postgres, a local Pub/Sub broker, local object storage, your services with type-safe API calls between them, plus a [local dashboard](/docs/ts/observability/dev-dash) with distributed tracing, logs, and a database explorer. No configuration, no Docker Compose to maintain, no manual seeding of dependencies.

For agents running in parallel (one agent per task, one agent per branch), [infrastructure namespaces](/docs/ts/cli/infra-namespaces) give each branch or task its own isolated local state. `encore namespace switch --create pr:123` creates a fresh namespace with its own database; switching back later restores the previous state.

## Standardize how infrastructure is integrated

Encore's compiler validates how your code uses each declared resource. There is one governed way to integrate a database, publish to a topic, expose an API, or read a secret. Service discovery, connection strings, and other glue are generated deterministically.

For AI agents this matters twice over: the surface area an agent has to learn is small and consistent, and the compiler catches mistakes the moment the agent makes them, rather than after a deploy.

## Validate changes end-to-end in per-PR preview environments

When you open a pull request, Encore Cloud automatically spins up a [preview environment](/docs/platform/deploy/preview-environments) in your own VPC. It comes up in minutes and runs the same infrastructure model as production, in real cloud services.

You can [branch the database from a seed environment](/docs/platform/infrastructure/neon) so each PR starts with realistic data. Agents (and reviewers) can hit a real URL, run real requests, and observe real traces before any change is merged.

This is the iteration loop that's structurally missing from Terraform-style workflows: a way to verify "does this change actually work, end-to-end, against real infrastructure" without touching production.

## Self-serve cloud infrastructure on AWS or GCP

When a change is merged, the same model that ran locally and in the preview environment provisions production resources in your AWS or GCP account. Developers (and agents) can introduce new infrastructure, like a new database or Pub/Sub topic, by writing it in code; Encore Cloud creates the matching cloud resource on deploy. No separate Terraform PR required.

[Encore Cloud](/docs/platform) manages scaling, resource settings, and infrastructure configuration from one control plane, while keeping full access through your cloud console. Changes stay synced in both directions. [Least-privilege IAM and firewall rules](/docs/platform/deploy/security) are derived from how your code actually uses each resource, not hand-written policies.

## Why this matters for AI agents

Take the four properties together:

1. **Local infrastructure that mirrors production**, so an agent can run and observe its change immediately.
2. **A small, compiler-validated surface area**, so an agent picks from a stable vocabulary and gets fast feedback on mistakes.
3. **Per-PR preview environments**, so an agent can validate end-to-end against real cloud services before asking a human to review.
4. **Self-serve cloud provisioning from code**, so the agent's working artifact (code) is the same thing that goes to production.

Each stage gives the agent a way to verify rather than guess. The loop is fast end-to-end, which is what AI coding tools need to be useful on real backend work.

## Where to go next

- Start with the [Quick Start for TypeScript](/docs/ts/quick-start) or [Go](/docs/go/quick-start).
- See [Local Development Dashboard](/docs/ts/observability/dev-dash) and [Infrastructure Namespaces](/docs/ts/cli/infra-namespaces) for the local loop.
- See [Preview Environments](/docs/platform/deploy/preview-environments) and [Deploying & CI/CD](/docs/platform/deploy/deploying) for the cloud loop.
- See [AI Integration](/docs/platform/ai-integration) for the AI-specific tooling (instructions, MCP server, Cloud MCP).
