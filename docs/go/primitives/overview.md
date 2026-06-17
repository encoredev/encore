---
seotitle: Encore.go Primitives — Infrastructure resources for Go
seodesc: An overview of the cloud infrastructure primitives Encore.go gives you, including SQL databases, Pub/Sub, object storage, caches, cron jobs, and secrets.
title: Primitives
subtitle: The infrastructure resources you can declare in your Encore.go application
lang: go
---

Encore.go gives you the core set of infrastructure primitives that backend applications reach for 99% of the time: SQL databases, Pub/Sub, object storage, caches, cron jobs, and secrets. You declare them directly in your Go code as typed values and use them through their methods.

When you run your application locally, `encore run` starts a matching local implementation of each primitive (real Postgres, a local Pub/Sub broker, local object storage, and so on). In production you have two options. With [Encore Cloud](/docs/platform), the same declarations are used to provision the equivalent managed services (RDS, SNS+SQS, S3, etc.) in your own AWS or GCP account. If you prefer to self-host, you provision the infrastructure yourself (using Terraform or any other tool) and point Encore at it through an [infrastructure config file](/docs/go/self-host/configure-infra).

## A standard toolbox for developers and AI agents

These primitives also make Encore.ts an ideal target for efficient development work, especially using AI coding agents. Almost any backend problem can be solved by composing this small, well-understood set of building blocks, so humans and agents don't need to evaluate dozens of competing libraries or assemble bespoke infrastructure for each task. Instead, you pick from a stable, typed vocabulary that maps directly to production cloud resources. The infrastructure building blocks capture the semantics of the infrastructure resources used, which means you can reason about the full stack from a single source of truth.

## Application building blocks

These are the structural primitives that organize your code.

- **[App Structure](/docs/go/primitives/app-structure)** — How an Encore application is laid out, and how services fit together in a monorepo.
- **[Services](/docs/go/primitives/services)** — Group related APIs and infrastructure into independently deployable services.
- **[Defining APIs](/docs/go/primitives/defining-apis)** — Expose typed endpoints from a service. Encore handles request validation, routing, and client generation.
- **[API Calls](/docs/go/primitives/api-calls)** — Call another service's API as a regular typed function. Encore wires it up in-process locally and over the network in production.

For more advanced API styles, see [Raw Endpoints](/docs/go/primitives/raw-endpoints), [Service Structs](/docs/go/primitives/service-structs), and [API Errors](/docs/go/primitives/api-errors).

## Data and storage

- **[SQL Databases](/docs/go/primitives/databases)** — Declare a PostgreSQL database, manage migrations, and run queries. Provisioned as RDS or Cloud SQL in production.
- **[Object Storage](/docs/go/primitives/object-storage)** — Store and serve files. Backed by a local filesystem in development and S3 or GCS in production.
- **[Caching](/docs/go/primitives/caching)** — Typed Redis-backed caches with structured key and value types.

## Asynchronous work

- **[Pub/Sub](/docs/go/primitives/pubsub)** — Publish typed events and subscribe to them from other services. Backed by SNS+SQS on AWS and Pub/Sub on GCP.
- **[Cron Jobs](/docs/go/primitives/cron-jobs)** — Run an API endpoint on a recurring schedule.

## Configuration

- **[Secrets](/docs/go/primitives/secrets)** — Reference secret values by name in code; Encore stores them in your cloud's secret manager and injects them at runtime.

## How primitives map to your cloud

Encore reads your primitive declarations to build an infrastructure model of your application. That model is what drives both local development and cloud provisioning, so the resources you use in production are the ones your code asked for, nothing more, nothing less.

To see the cloud resources Encore creates from these primitives, see [Infrastructure on AWS and GCP](/docs/platform/infrastructure/infra).
