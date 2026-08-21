---
seotitle: "Encore Primitives: infrastructure resources you declare in code"
seodesc: An overview of the cloud infrastructure primitives Encore gives you, including SQL databases, Pub/Sub, object storage, caches, cron jobs, and secrets.
title: Primitives
subtitle: The infrastructure resources you can declare in your application
lang: ts
---

Encore gives you six infrastructure primitives, declared as typed objects in the same files as the code that uses them:

<CodeTabs>
<CodeTab label="SQL Database">
```ts
import { SQLDatabase } from "encore.dev/storage/sqldb";

const db = new SQLDatabase("orders", { migrations: "./migrations" });
```
</CodeTab>
<CodeTab label="Pub/Sub">
```ts
import { Topic } from "encore.dev/pubsub";

export const orderEvents = new Topic<OrderEvent>("order-events", {
  deliveryGuarantee: "at-least-once",
});
```
</CodeTab>
<CodeTab label="Object Storage">
```ts
import { Bucket } from "encore.dev/storage/objects";

export const invoices = new Bucket("invoices");
```
</CodeTab>
<CodeTab label="Caching">
```ts
import { CacheCluster } from "encore.dev/storage/cache";

const cluster = new CacheCluster("orders-cache", {
  evictionPolicy: "allkeys-lru",
});
```
</CodeTab>
<CodeTab label="Cron Jobs">
```ts
import { CronJob } from "encore.dev/cron";

const _ = new CronJob("send-invoices", {
  title: "Send pending invoices",
  every: "2h",
  endpoint: sendInvoices,
});
```
</CodeTab>
<CodeTab label="Secrets">
```ts
import { secret } from "encore.dev/config";

const stripeKey = secret("StripeAPIKey");
```
</CodeTab>
</CodeTabs>

You declare services and APIs in code the same way, usually in the same files, and they have their own section [further down](#services-and-apis).

A declaration says what the application needs and nothing about how the cloud should supply it. Encore reads these declarations into the [application model](/docs/ts/concepts/application-model), and that model is what drives provisioning, IAM and code generation.

Because nothing in a declaration is environment-specific, the same code runs against a local database, a preview environment's database and production's, and never has to check which one it is talking to. The [development workflow](/docs/platform/workflow) covers that loop, from `encore run` through a per-PR preview environment to production.

## What each primitive becomes

`encore run` starts a local implementation of each one, and in cloud environments [Encore Cloud](/docs/platform) provisions the managed equivalent in your own AWS or GCP account. You can also provision the infrastructure yourself and point Encore at it with an [infra config file](/docs/ts/self-host/configure-infra).

| Primitive | Locally | AWS | GCP |
|---|---|---|---|
| [SQL Database](/docs/ts/primitives/databases) | Postgres in Docker | RDS | Cloud SQL |
| [Pub/Sub](/docs/ts/primitives/pubsub) | in-memory | SNS + SQS | Cloud Pub/Sub |
| [Object Storage](/docs/ts/primitives/object-storage) | local filesystem | S3 | Cloud Storage |
| [Caching](/docs/ts/primitives/caching) | in-memory Redis | ElastiCache | Memorystore |
| [Cron Jobs](/docs/ts/primitives/cron-jobs) | not triggered | CloudWatch Events | Cloud Scheduler |
| [Secrets](/docs/ts/primitives/secrets) | set per environment | Secrets Manager | Secret Manager |

Cron jobs are the exception, and are not triggered in local or preview environments to avoid surprises, so you invoke the endpoint directly from the [development dashboard](/docs/ts/observability/dev-dash) instead.

## Services and APIs

These are not cloud resources, but the compiler reads them into the same model and checks them the same way.

- **[App Structure](/docs/ts/primitives/app-structure)**: how an application is laid out, and how services fit together in a monorepo.
- **[Services](/docs/ts/primitives/services)**: group related APIs and infrastructure into independently deployable services.
- **[Defining APIs](/docs/ts/primitives/defining-apis)**: expose typed endpoints from a service. Encore handles request validation, routing and client generation.
- **[API Calls](/docs/ts/primitives/api-calls)**: call another service's API as a regular typed function, wired up in-process locally and over the network in production.

### Request and response data

- **[Types](/docs/ts/primitives/types)**: how types map onto headers, query parameters and path parameters.
- **[Validation](/docs/ts/primitives/validation)**: incoming requests are checked against the declared schema before your handler runs.
- **[Errors](/docs/ts/primitives/errors)**: return structured errors with codes that map to HTTP statuses.
- **[Cookies](/docs/ts/primitives/cookies)**: typed cookie handling.

### Other API styles

- **[Raw Endpoints](/docs/ts/primitives/raw-endpoints)**: drop to the underlying request and response objects.
- **[Streaming APIs](/docs/ts/primitives/streaming-apis)**: streams in either direction over WebSockets.
- **[GraphQL](/docs/ts/primitives/graphql)**: serve a GraphQL schema from a service.
- **[Static Assets](/docs/ts/primitives/static-assets)**: serve files from a directory.

## Primitives and AI agents

Infrastructure is where agents go wrong most often, because a wrong property value usually applies without failing and a diff does not reveal it. Declaring a database as configuration means producing an instance class, a storage type, a subnet group, a security group, a parameter group and an IAM policy, any of which can be wrong in a way nothing catches.

The same database as a primitive is a name and a directory:

```ts
const db = new SQLDatabase("orders", { migrations: "./migrations" });
```

Everything the configuration version needed is either absent from the code or checked when you build:

- Instance sizing, networking and backups are [environment settings](/docs/platform/infrastructure/configuration), so they are not in the code an agent edits and cannot be hallucinated into it.
- Resource names have to be string literals and declarations have to sit at module scope, so a name assembled from a variable fails the build instead of half-working. The [application model](/docs/ts/concepts/application-model) page has the full set of requirements.
- IAM policies are derived from which services actually use which resources, so an agent cannot grant itself broader access than the code it wrote uses.
- Request schemas come from your types, so a wrong handler signature is a compile error rather than a bad response.

For the editor rules and MCP server that give agents the service graph, schemas and traces, see [AI integration](/docs/ts/ai-integration). If you are coming from Terraform, [Coming from Terraform](/docs/platform/migration/from-terraform) maps the concepts across and covers running both alongside each other.

## Anything the primitives don't cover

A search cluster, a data warehouse, a queue with semantics none of these have: provision it however you like and reach it from your code the way you would any external dependency, with its connection details in a [secret](/docs/ts/primitives/secrets).

To see exactly what Encore creates in your cloud, see [Infrastructure on AWS and GCP](/docs/platform/infrastructure/infra).
