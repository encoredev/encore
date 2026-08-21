---
seotitle: "What is Encore?"
seodesc: Encore is a platform for building backend applications and running them in your own AWS or GCP account, built around an open source SDK that declares infrastructure in your application code.
title: What is Encore?
subtitle: A platform for building backend applications and running them in your own cloud
lang: platform
---

Encore provisions the infrastructure your services declare and runs it in your own AWS or GCP account. You add it to a service as an SDK, so an existing service keeps its structure.

An application contains one or more [services](/docs/ts/primitives/app-structure), and a service holds its APIs alongside the infrastructure it needs. For example, a SQL database:

<CodeTabs>
<CodeTab label="TypeScript">
```ts
import { SQLDatabase } from "encore.dev/storage/sqldb";

const db = new SQLDatabase("orders", { migrations: "./migrations" });
```
</CodeTab>
<CodeTab label="Go">
```go
import "encore.dev/storage/sqldb"

var db = sqldb.NewDatabase("orders", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
```
</CodeTab>
</CodeTabs>

Encore's compiler reads that declaration into an [application model](/docs/ts/concepts/application-model): a description of every service, resource and API in your application. From that model Encore creates the database, as a Postgres container on your laptop and as RDS or Cloud SQL in your cloud account. You choose the instance size, engine and cloud [per environment](/docs/platform/infrastructure/configuration), in the dashboard. The other [primitives](/docs/ts/primitives) work the same way: Pub/Sub topics, object storage, caches, cron jobs and secrets.

`encore run` starts every service in your application, along with the infrastructure they declare and a [dashboard](/docs/ts/observability/dev-dash) with tracing, logs and a database explorer. Because that environment comes from the same model, it changes when your code does. The [development workflow](/docs/platform/workflow) covers the loop from here through per-PR preview environments to production.

The model also records which operations each service performs on each resource, and Encore derives each service's IAM policy, the typed clients for your APIs and the service catalog from those records.

## What Encore is made of

- You write against the Infra SDK, in [TypeScript](/docs/ts) or [Go](/docs/go). It covers six infrastructure [primitives](/docs/ts/primitives), and it is open source along with the parser, compiler, CLI and runtimes.
- The compiler produces the [application model](/docs/ts/concepts/application-model) from your code, recording which services exist, which resources each one owns, and which operations each one performs.
- `encore run` starts your whole application on your laptop, with a local implementation of every resource it declares and a [development dashboard](/docs/ts/observability/dev-dash) for tracing, logs and a database explorer.
- Encore provisions [cloud infrastructure](/docs/platform/infrastructure/infra) in your own AWS or GCP account, applies your [per-environment settings](/docs/platform/infrastructure/configuration), and derives [least-privilege IAM and firewall rules](/docs/platform/deploy/security) from how your code uses each resource.
- Encore [deploys](/docs/platform/deploy/deploying) to production and to a [preview environment](/docs/platform/deploy/preview-environments) for every pull request, each running the same infrastructure model.
- [Distributed tracing](/docs/ts/observability/tracing) and the [service catalog](/docs/ts/observability/service-catalog) come from the model, so neither drifts from the code it describes.

You can also run an Encore application without the platform. `encore build docker` produces a standard image, and an [infra config file](/docs/ts/self-host/configure-infra) tells the runtime how to reach infrastructure you provisioned yourself.

## Working with AI agents

The properties that make the model useful to a compiler serve an agent as well. An agent never edits the settings that can be misconfigured, the compiler checks resource names and scope at build time, and permissions follow actual usage, so an agent cannot grant itself access the code it wrote does not use.

[Infrastructure namespaces](/docs/ts/cli/infra-namespaces) give each agent or branch its own isolated local state, which matters once several run at once. [AI integration](/docs/ts/ai-integration) covers the editor rules and MCP server that hand an agent your service graph, schemas and traces, and [AI infrastructure provisioning](/docs/platform/ai-integration) covers the guardrails on what it can create in your cloud account.

## Where each setting lives

Encore splits configuration across three places, and knowing which is which answers most questions about what you can still control.

| Where | What it decides |
|---|---|
| Your code | Which services exist, which resources they own, and which operations they perform on them |
| [The Encore dashboard](/docs/platform/infrastructure/configuration) | Cloud provider, compute platform, database engine, instance sizes and process allocation, per environment |
| [An infra config file](/docs/ts/self-host/configure-infra) | Which infrastructure you provisioned yourself each logical resource maps to, when running without the platform |

Nothing environment-specific belongs in the first row, so the same code runs in every environment.

## Start here

- Build something: the Quick Start for [TypeScript](/docs/ts/quick-start) or [Go](/docs/go/quick-start).
- Understand the mechanism: the [application model](/docs/ts/concepts/application-model) explains how code becomes infrastructure, and [Primitives](/docs/ts/primitives) covers each resource you can declare.
- Coming from another tool: [Coming from an IaC tool](/docs/platform/migration/from-iac) or [Coming from a PaaS](/docs/platform/migration/from-paas).
- Running it alongside what you already have: Encore can import existing [RDS](/docs/platform/infrastructure/aws/import-rds), [Cloud SQL](/docs/platform/infrastructure/gcp/import-cloud-sql), [S3](/docs/platform/infrastructure/aws/import-s3-bucket) and [Kubernetes](/docs/platform/infrastructure/import-kubernetes-cluster) resources instead of recreating them, and the [Terraform Provider](/docs/platform/integrations/terraform) reads Encore-provisioned resources back into your own configuration.
- Evaluating what the platform runs: [Platform](/docs/platform) covers provisioning, environments and operations in more depth.

## What Encore doesn't do

For anything outside the six primitives, such as a search cluster or a data warehouse, you provision it yourself and reach it from your code as an external dependency, with its connection details in a [secret](/docs/ts/primitives/secrets).

Infrastructure whose shape depends on runtime data also sits outside the model, because Encore builds the model by reading your code rather than running it. A resource created inside a conditional, or from a computed name, never reaches the model at all.
