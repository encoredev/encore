---
seotitle: The Encore Development Workflow — A tight iteration loop for humans and AI agents
seodesc: Encore gives you the same infrastructure model from local development, through per-PR preview environments, to production on AWS or GCP. The fast feedback loop makes it especially well-suited to AI coding agents.
title: Development Workflow
subtitle: A tight iteration loop from your laptop to AWS or GCP, designed for fast feedback with humans and AI agents
lang: platform
---

An Encore application runs in three places: on your laptop, in a preview environment for every pull request, and in production in your own AWS or GCP account. The same infrastructure model drives all three, because that model comes from your application code rather than from a separate configuration codebase. Nothing cloud-specific appears in that code: instance sizes, database engines and compute hardware are per-environment [infrastructure settings](/docs/platform/infrastructure/configuration) you choose in the Encore dashboard, so staging can run a small shared database while production runs a large managed one.

When infrastructure is defined outside the application it only applies against cloud accounts, so teams approximate the local half with Docker Compose files, hand-rolled mocks and staging-only testing, and those approximations drift from what production actually does. For a fuller comparison of the two approaches, see [Coming from Terraform](/docs/platform/migration/from-terraform).

Because Encore's model runs anywhere, a change can be run, tested and observed end-to-end before it is merged, and most questions about whether it works get answered before a deploy instead of after one. Humans benefit from that, and AI coding agents depend on it, since an agent is only as useful as its ability to check its own work.

## What `encore run` starts

With Encore you declare infrastructure (SQL databases, Pub/Sub, object storage, caches, cron jobs, secrets) as objects in your application code using the open source Encore [TypeScript](/docs/ts) or [Go](/docs/go) SDK.

`encore run` starts the whole system: Postgres in a Docker container, a local Pub/Sub broker, object storage on your filesystem, your services with type-safe API calls between them, plus a [local dashboard](/docs/ts/observability/dev-dash) with distributed tracing, logs, and a database explorer. Encore derives that setup from your declarations, so your repo carries no Compose file or emulator config that has to be kept in sync with what production runs.

<video autoPlay playsInline loop muted className="w-full h-auto">
  <source src="/assets/docs/localdevdash.mp4" type="video/mp4"/>
</video>

The model is identical across environments; the implementations underneath it are not. In production that local Postgres container is RDS or Cloud SQL, or the local Pub/Sub broker becomes SNS and SQS or Cloud Pub/Sub, for example. Everything you write stays the same: the same declarations, the same generated clients, the same API surface, with nothing in your code that has to ask which environment it is running in. [Infrastructure on AWS and GCP](/docs/platform/infrastructure/infra) lists what each primitive becomes in each environment.

Cron jobs are the one deliberate exception, and do not fire locally or in preview environments so that a schedule cannot surprise you while you are working; you invoke the endpoint from the dashboard instead.

For agents running in parallel (one agent per task, one agent per branch), [infrastructure namespaces](/docs/ts/cli/infra-namespaces) give each branch or task its own isolated local state. `encore namespace switch --create pr:123` creates a fresh namespace with its own database; switching back later restores the previous state.

## What `encore test` sets up

`encore test` provisions the infrastructure in test mode and then hands off to your test runner: Vitest or Jest for TypeScript, `go test` for Go. Each run gets its own databases, tuned for speed over durability by skipping `fsync` and using in-memory filesystems, and object storage runs in memory as well.

Encore removes most of the boilerplate, so what is left to test is mostly business logic over databases and calls between services, which is what integration tests verify. Encore applications typically lean on them for that reason, and the test-mode setup described in [Automated testing](/docs/ts/develop/testing) makes them nearly as fast as unit tests.

## What a preview environment runs

When you open a pull request, Encore automatically spins up a [preview environment](/docs/platform/deploy/preview-environments) in your own VPC. It comes up in minutes and runs the same infrastructure model as production, in real cloud services.

You can [branch the database from a seed environment](/docs/platform/infrastructure/neon) so each PR starts with realistic data. Reviewers and agents get a real URL, can send real requests, and can read the resulting traces before anything is merged.

## Production in your own cloud account

When a change is merged, the same model that ran locally and in the preview environment provisions production resources in your AWS or GCP account. New infrastructure, like another database or Pub/Sub topic, is introduced by writing it in code, and the matching cloud resource is created on deploy without a separate Terraform PR.

Those same per-environment settings are managed from one control plane, while you keep full access through your cloud console and changes stay synced in both directions. [Least-privilege IAM and firewall rules](/docs/platform/deploy/security) are derived from how your code actually uses each resource, rather than hand-written.

## How your code uses each resource

Encore's compiler validates how your code uses each declared resource. There is one governed way to query a database or publish to a topic, the same in every environment:

```ts
// orderEvents is a Topic, declared the same way as the database above
for (const order of await db.queryAll`SELECT id FROM orders WHERE status = 'open'`) {
  await orderEvents.publish({ orderID: order.id });
}
```

Service discovery, connection strings, and other glue are generated deterministically from those usages, so misusing a resource is a build error rather than a runtime surprise. Those same usages are what the [application model](/docs/ts/concepts/application-model) records, and where the IAM policies above come from.

## The workflow and AI agents

Each stage of this loop gives an agent a way to check its own work.

1. **Local infrastructure that matches the production model**, so an agent can run and observe a change immediately.
2. **A test command that provisions its own infrastructure**, so a change can be proven to work without a deploy.
3. **A small, compiler-validated surface area**, so mistakes surface as build errors in seconds.
4. **Per-PR preview environments**, so end-to-end validation against real cloud services happens before a human reviews.
5. **Provisioning from code**, so the artifact the agent produces is the same thing that goes to production.

[AI Integration](/docs/ts/ai-integration) covers the editor rules and MCP server that give an agent your service graph, schemas and traces. [AI infrastructure provisioning](/docs/platform/ai-integration) covers the guardrails on what an agent can create in your cloud account.

## Where to go next

- Start with the [Quick Start for TypeScript](/docs/ts/quick-start) or [Go](/docs/go/quick-start).
- See [Local Development Dashboard](/docs/ts/observability/dev-dash) and [Infrastructure Namespaces](/docs/ts/cli/infra-namespaces) for the local loop.
- See [Automated testing](/docs/ts/develop/testing) for running tests against real infrastructure.
- See [Preview Environments](/docs/platform/deploy/preview-environments) and [Deploying & CI/CD](/docs/platform/deploy/deploying) for the cloud loop.
