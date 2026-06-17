---
seotitle: Introduction to Encore Cloud
seodesc: Encore Cloud is the managed platform that pairs with the Encore.ts and Encore.go infrastructure SDKs to provision and operate AWS and GCP from your code.
title: Encore Cloud
subtitle: The managed platform that provisions and operates AWS and GCP from your application code
lang: platform
---

Cloud platforms are powerful, but the path from code to a running production system on AWS or GCP usually involves Terraform, YAML, container orchestration, IAM policies, secret managers, and a CI/CD pipeline to glue it all together. Most teams end up building (and maintaining) their own internal developer platform on top of all that.

Encore Cloud removes the need for that internal platform. It pairs with the open source [Encore.ts](/docs/ts) and [Encore.go](/docs/go) infrastructure SDKs to take an application from local development to a production deployment in your own cloud, with the SDK declarations as the single source of truth.

## How Encore Cloud works with the SDKs

The infrastructure SDKs let you declare the resources your application needs (SQL databases, Pub/Sub topics, object storage, caches, cron jobs, secrets) directly in your TypeScript or Go code, as typed objects.

Encore Cloud reads those declarations to build an infrastructure model of your application. It then uses that model to:

1. Provision matching resources in your AWS or GCP account using the cloud provider's API.
2. Wire the running application up to those resources at deploy time (connection strings, IAM, secret injection).
3. Keep the model and the running infrastructure in sync as your code changes.

The resources used in production are exactly the ones your code asked for, nothing more, nothing less.

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/howitworks.png" title="How Encore Cloud provisions infrastructure from your code" />

When your application is deployed, **there are no runtime dependencies on Encore Cloud** and **no proprietary code runs in your cloud**. The application is a standard service talking to standard managed AWS or GCP resources.

## A tight iteration loop, local to production

Because the SDK is the source of truth for your infrastructure, the same model runs locally, in per-PR preview environments, and in production:

1. **Local**. `encore run` boots the whole system on your laptop with real Postgres, real Pub/Sub semantics, and real tracing. [Infrastructure namespaces](/docs/ts/cli/infra-namespaces) let multiple branches or agents work in parallel with isolated local state.
2. **Per-PR preview environments**. Each pull request gets an ephemeral [preview environment](/docs/platform/deploy/preview-environments) in your own VPC, optionally with a [database branched from a seed environment](/docs/platform/infrastructure/neon). End-to-end validation against real cloud services before merge.
3. **Production**. The same declarations provision the production resources on deploy.

This loop is what's structurally missing from Terraform-based workflows, which tend to be production-centric and hard to run on a laptop. The tight feedback loop is especially impactful for [AI coding agents](/docs/platform/ai-integration), which work best when they can validate their own changes against real infrastructure rather than guess. See the [Development Workflow](/docs/platform/workflow) page for the full picture.

## What Encore Cloud gives you

### Infrastructure provisioning in your own cloud
- **Direct provisioning via cloud APIs**. Encore Cloud creates resources using your cloud provider's official APIs. No IaC files to maintain; your code is the source of truth. See [Provisioning & Environments](/docs/platform/infrastructure/infra).
- **Battle-tested managed services**. Provisions Cloud Run / Fargate, GKE / EKS, CloudSQL / RDS, Pub/Sub / SNS+SQS, and so on, with sensible defaults per cloud.
- **Least-privilege IAM, generated from code**. Permissions are derived from how each resource is used, not hand-written policies.
- **Infrastructure inventory and change management**. See every provisioned resource and review infrastructure changes before they're applied.

### Deployments and environments
- **Zero-config deploys**. Connect a [GitHub repo](/docs/platform/integrations/github) and your cloud account; push to deploy. See [Deploying & CI/CD](/docs/platform/deploy/deploying).
- **[Preview environments](/docs/platform/deploy/preview-environments) per pull request**. Real cloud resources, automatically created and torn down.
- **Multiple environments out of the box**. Staging, production, and ad-hoc environments share the same code path. See [Environments](/docs/platform/deploy/environments).
- **Secrets management**. Reference secrets by name in code; Encore stores them in your cloud's secret manager and injects them at runtime.

### Observability
- **Distributed tracing, metrics, and logs**, with no agent code in your application. See [Tracing](/docs/platform/observability/tracing) and [Metrics](/docs/platform/observability/metrics).
- **Third-party export**. Send telemetry to Datadog, Grafana, or other tools you already use.
- **Cost monitoring** across your cloud resources (GCP today, AWS in progress).

### Documentation and discovery
The infrastructure model also drives a [Service Catalog](/docs/platform/observability/service-catalog) with API docs and a live [Flow architecture diagram](/docs/platform/observability/encore-flow). Useful for onboarding new engineers and for AI tooling, but not the reason most teams adopt Encore.

## You're not locked in

Encore Cloud is optional. The SDKs are open source and produce a standard application binary or Node.js process. If you want to run things yourself:

- Build a Docker image with `encore build docker` and deploy it anywhere.
- Provision your own infrastructure with [Terraform](/docs/platform/integrations/terraform), Pulumi, CloudFormation, or the cloud console.
- Tell the Encore runtime how to reach your resources via an [infrastructure config file](/docs/ts/self-host/configure-infra).

See the [self-hosting guide](/docs/ts/self-host/build) for the full workflow.

## When Encore Cloud is a good fit

Encore Cloud suits teams that want their application's infrastructure to be defined and operated from the same codebase as the application itself, deployed into AWS or GCP accounts they own. Typical use cases include:

- Consumer apps and B2B platforms
- Fintech and crypto applications
- E-commerce marketplaces
- Microservices backends and event-driven systems

See the [showcase](https://encore.cloud/showcase) for production examples, or read [customer stories](https://encore.cloud/customers) for how specific teams use it.

## Getting started

- Start with the [Quick Start for TypeScript](/docs/ts/quick-start) or [Go](/docs/go/quick-start).
- [Connect your AWS or GCP account](/docs/platform/deploy/own-cloud).
- [Book a demo](https://encore.dev/book) to talk through whether Encore Cloud fits your team.
- [Join Discord](https://encore.dev/discord) to ask questions and meet other Encore developers.
