---
seotitle: "Encore Primitives: infrastructure resources you declare in code"
seodesc: An overview of the cloud infrastructure primitives Encore gives you, including SQL databases, Pub/Sub, object storage, caches, cron jobs, and secrets.
title: Primitives
subtitle: The infrastructure resources you can declare in your application
lang: go
---

Encore gives you six infrastructure primitives, declared as package-level variables in the same files as the code that uses them:

<CodeTabs>
<CodeTab label="SQL Database">
```go
import "encore.dev/storage/sqldb"

var db = sqldb.NewDatabase("orders", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
```
</CodeTab>
<CodeTab label="Pub/Sub">
```go
import "encore.dev/pubsub"

var OrderEvents = pubsub.NewTopic[*OrderEvent]("order-events", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})
```
</CodeTab>
<CodeTab label="Object Storage">
```go
import "encore.dev/storage/objects"

var Invoices = objects.NewBucket("invoices", objects.BucketConfig{})
```
</CodeTab>
<CodeTab label="Caching">
```go
import "encore.dev/storage/cache"

var OrdersCache = cache.NewCluster("orders-cache", cache.ClusterConfig{
	EvictionPolicy: cache.AllKeysLRU,
})
```
</CodeTab>
<CodeTab label="Cron Jobs">
```go
import "encore.dev/cron"

var _ = cron.NewJob("send-invoices", cron.JobConfig{
	Title:    "Send pending invoices",
	Every:    2 * cron.Hour,
	Endpoint: SendInvoices,
})
```
</CodeTab>
<CodeTab label="Secrets">
```go
var secrets struct {
	StripeAPIKey string // API key for the payments provider
}
```
</CodeTab>
</CodeTabs>

Services and APIs are declared in code the same way, usually in the same files, and are covered in [Services and APIs](#services-and-apis) further down.

A declaration says what the application needs and nothing about how the cloud should supply it. Encore reads these declarations into the [application model](/docs/go/concepts/application-model), and that model is what drives provisioning, IAM and code generation.

Because nothing in a declaration is environment-specific, the same code runs against a local database, a preview environment's database and production's, and never has to check which one it is talking to. That loop, from `encore run` through a per-PR preview environment to production, is covered in the [development workflow](/docs/platform/workflow).

## What each primitive becomes

`encore run` starts a local implementation of each one, and in cloud environments [Encore Cloud](/docs/platform) provisions the managed equivalent in your own AWS or GCP account. You can also provision the infrastructure yourself and point Encore at it with an [infra config file](/docs/go/self-host/configure-infra).

| Primitive | Locally | AWS | GCP |
|---|---|---|---|
| [SQL Database](/docs/go/primitives/databases) | Postgres in Docker | RDS | Cloud SQL |
| [Pub/Sub](/docs/go/primitives/pubsub) | in-memory | SNS + SQS | Cloud Pub/Sub |
| [Object Storage](/docs/go/primitives/object-storage) | local filesystem | S3 | Cloud Storage |
| [Caching](/docs/go/primitives/caching) | in-memory Redis | ElastiCache | Memorystore |
| [Cron Jobs](/docs/go/primitives/cron-jobs) | not triggered | CloudWatch Events | Cloud Scheduler |
| [Secrets](/docs/go/primitives/secrets) | set per environment | Secrets Manager | Secret Manager |

Cron jobs are the exception, and are not triggered in local or preview environments to avoid surprises, so you invoke the endpoint directly from the [development dashboard](/docs/go/observability/dev-dash) instead.

## Services and APIs

These are not cloud resources, but the compiler reads them into the same model and checks them the same way.

- **[App Structure](/docs/go/primitives/app-structure)**: how an application is laid out, and how services fit together in a monorepo.
- **[Services](/docs/go/primitives/services)**: group related APIs and infrastructure into independently deployable services.
- **[Defining APIs](/docs/go/primitives/defining-apis)**: expose typed endpoints from a service. Encore handles request validation, routing and client generation.
- **[API Calls](/docs/go/primitives/api-calls)**: call another service's API as a regular typed function, wired up in-process locally and over the network in production.

### Request and response data

- **[Validation](/docs/go/develop/validation)**: incoming requests are checked against the declared schema before your handler runs.
- **[API Errors](/docs/go/primitives/api-errors)**: return structured errors with codes that map to HTTP statuses.

### Other API styles

- **[Raw Endpoints](/docs/go/primitives/raw-endpoints)**: drop to the underlying request and response objects.
- **[Service Structs](/docs/go/primitives/service-structs)**: define APIs as methods on a struct, with dependencies initialized once per service.

## Primitives and AI agents

Infrastructure is where agents go wrong most often, because a wrong property value usually applies without failing and a diff does not reveal it. Declaring a database as configuration means producing an instance class, a storage type, a subnet group, a security group, a parameter group and an IAM policy, any of which can be wrong in a way nothing catches.

The same database as a primitive is a name and a directory:

```go
var db = sqldb.NewDatabase("orders", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
```

Everything the configuration version needed is either absent from the code or checked when you build:

- Instance sizing, networking and backups are [environment settings](/docs/platform/infrastructure/configuration), so they are not in the code an agent edits and cannot be hallucinated into it.
- Resource names have to be string literals and declarations have to sit at package level, so a name assembled from a variable fails the build instead of half-working. The [application model](/docs/go/concepts/application-model) page has the full set of requirements.
- IAM policies are derived from which services actually use which resources, so an agent cannot grant itself broader access than the code it wrote uses.
- Request schemas come from your struct types, so a wrong handler signature is a compile error rather than a bad response.

For the editor rules and MCP server that give agents the service graph, schemas and traces, see [AI integration](/docs/go/ai-integration). If you are coming from Terraform, [Coming from Terraform](/docs/platform/migration/from-terraform) maps the concepts across and covers running both alongside each other.

## Anything the primitives don't cover

A search cluster, a data warehouse, a queue with semantics none of these have: provision it however you like and reach it from your code the way you would any external dependency, with its connection details in a [secret](/docs/go/primitives/secrets).

To see exactly what Encore creates in your cloud, see [Infrastructure on AWS and GCP](/docs/platform/infrastructure/infra).
