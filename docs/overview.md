---
seotitle: "What is Encore?"
seodesc: Encore is a platform for building backend applications and running them in your own AWS or GCP account, built around an open source SDK that declares infrastructure in your application code.
title: What is Encore?
subtitle: A platform for building backend applications and running them in your own cloud
lang: platform
---

Encore is a platform for building backend applications and running them on AWS or GCP. It provisions the infrastructure your services declare, the resources you would otherwise write Terraform for, and runs the same setup on your laptop as in your own cloud account. You add it as an SDK rather than rewriting your services into it, and you keep control of how every resource is configured.

An Encore application is one or more [services](/docs/ts/primitives/app-structure), and it is not opinionated about monoliths or microservices. A service holds its APIs and the infrastructure it owns, declared in the code that uses it.

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

Encore's compiler reads that declaration and provisions a real Postgres database: a container on your laptop, RDS or Cloud SQL in production. The declaration says nothing about instance size, engine or which cloud it runs in, because those are [per-environment settings](/docs/platform/infrastructure/configuration) you set in the dashboard, or in a config file if you run Encore yourself.

`encore run` starts the whole application locally: the database, a Pub/Sub broker, your services calling each other, and a [dashboard](/docs/ts/observability/dev-dash) with tracing, logs and a database explorer. There is no local setup of your own to maintain.

Encore also records which operations your code performs on that database, and calls the result the application model. Each service's IAM policy, the typed clients for your APIs and the service catalog are derived from that usage rather than written by you. Tools for writing infrastructure in a real language cannot do that last part.

## What Encore is made of

- The Infra SDK is what you write against, in [TypeScript](/docs/ts) or [Go](/docs/go). It covers six infrastructure [primitives](/docs/ts/primitives), and it is open source along with the parser, compiler, CLI and runtimes.
- The [application model](/docs/ts/concepts/application-model) is what the compiler produces from your code. It records which services exist, which resources each one owns, and which operations each one performs.
- Local development runs the whole system on your laptop. `encore run` starts a local implementation of every resource you declared, with a [development dashboard](/docs/ts/observability/dev-dash) for tracing, logs and a database explorer.
- [Cloud infrastructure](/docs/platform/infrastructure/infra) is provisioned in your own AWS or GCP account, with [per-environment settings](/docs/platform/infrastructure/configuration) and [least-privilege IAM and firewall rules](/docs/platform/deploy/security) derived from how your code actually uses each resource.
- [Deploys and environments](/docs/platform/deploy/deploying) cover production and a [preview environment](/docs/platform/deploy/preview-environments) for every pull request, running the same infrastructure model as production.
- [Observability](/docs/ts/observability/tracing) gives you distributed tracing and a [service catalog](/docs/ts/observability/service-catalog) built from the model, so neither can drift from the code it describes.

You can also run an Encore application without the platform. `encore build docker` produces a standard image, and an [infra config file](/docs/ts/self-host/configure-infra) tells the runtime how to reach infrastructure you provisioned yourself.

## Working with AI agents

The properties that make the model useful to a compiler make it useful to an agent as well. The settings that can be misconfigured are not in the code an agent edits, resource names and scope are checked when you build, and permissions follow actual usage, so an agent cannot grant itself access the code it wrote does not use.

[Infrastructure namespaces](/docs/ts/cli/infra-namespaces) give each agent or branch its own isolated local state, which matters once several are running at once. [AI integration](/docs/ts/ai-integration) covers the editor rules and MCP server that give an agent your service graph, schemas and traces, and [AI infrastructure provisioning](/docs/platform/ai-integration) covers the guardrails on what an agent is allowed to create in your cloud account.

## Where each setting lives

Three separate places, and knowing which is which answers most questions about what you can still control.

| Where | What it decides |
|---|---|
| Your code | Which services exist, which resources they own, and which operations they perform on them |
| [The Encore dashboard](/docs/platform/infrastructure/configuration) | Cloud provider, compute platform, database engine, instance sizes and process allocation, set per environment |
| [An infra config file](/docs/ts/self-host/configure-infra) | Which infrastructure you provisioned yourself each logical resource maps to, when running without the platform |

Nothing environment-specific belongs in the first row, which is what lets the same code run in every environment. The loop that carries it from `encore run` through a preview environment to production is covered in the [development workflow](/docs/platform/workflow).

## Start here

- Build something: the Quick Start for [TypeScript](/docs/ts/quick-start) or [Go](/docs/go/quick-start).
- Understand the mechanism: the [application model](/docs/ts/concepts/application-model) explains how code becomes infrastructure, and [Primitives](/docs/ts/primitives) covers each resource you can declare.
- Coming from another tool: [Coming from an IaC tool](/docs/platform/migration/from-iac) or [Coming from a PaaS](/docs/platform/migration/from-paas).
- Running it alongside what you already have: existing resources can be imported rather than recreated, including [RDS](/docs/platform/infrastructure/aws/import-rds), [Cloud SQL](/docs/platform/infrastructure/gcp/import-cloud-sql), [S3](/docs/platform/infrastructure/aws/import-s3-bucket) and a [Kubernetes cluster](/docs/platform/infrastructure/import-kubernetes-cluster), and the [Terraform Provider](/docs/platform/integrations/terraform) reads Encore-provisioned resources back into your own configuration.
- Evaluating what the platform runs: [Platform](/docs/platform) covers provisioning, environments and operations in more depth.

## What Encore doesn't do

A search cluster, a data warehouse or a queue with semantics none of the six primitives have is something you provision however you like, and reach from your code as an external dependency with its connection details in a [secret](/docs/ts/primitives/secrets).

Infrastructure whose shape depends on runtime data also sits outside the model, because the model is built by reading your code rather than by running it. A resource created inside a conditional or from a computed name is not part of it.
