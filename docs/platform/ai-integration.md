---
seotitle: AI-Powered Development with Encore Cloud
seodesc: Learn how Encore Cloud enables AI agents to provision infrastructure in AWS/GCP with automatic guardrails, preview environments, and more.
title: AI Integration
subtitle: AI agents that can provision real infrastructure in your cloud
lang: platform
---

Encore Cloud supercharges AI-powered development by letting AI agents provision real infrastructure in your AWS or GCP account with automatic guardrails.

When you connect your cloud account to [Encore Cloud](https://encore.cloud), AI-generated code that declares databases, pub/sub topics, cron jobs, and other [primitives](/docs/ts/primitives) gets automatically provisioned with production-ready defaults: proper networking, IAM permissions, and security configurations.

<video autoPlay playsInline loop controls muted className="w-full h-full">
  <source src="/assets/docs/aws-demo.mp4" type="video/mp4" />
</video>

## How It Works

1. **AI writes infrastructure code** using Encore's declarative primitives
2. **Push to GitHub** to trigger a deployment
3. **Encore Cloud provisions** the infrastructure in your AWS/GCP account with automatic guardrails
4. **Preview environments** let you test AI-generated changes in isolation

This enables fast iterative development: AI generates code, you push it and validate in preview environments, then deploy seamlessly with automatic infrastructure provisioning.

## Infrastructure with Guardrails

When AI declares infrastructure using Encore primitives, Encore Cloud provisions it in your cloud with automatic guardrails:

- **Databases**: Proper networking, encryption at rest, automated backups
- **Pub/Sub**: Dead letter queues, retry policies, proper IAM roles
- **Secrets**: Encrypted storage, access controls
- **Services**: Load balancing, health checks, auto-scaling

AI doesn't need to know the intricacies of AWS or GCP. It just declares what it needs, and Encore Cloud handles the cloud-specific configuration.

You stay in control: review infrastructure changes in pull requests, approve or deny resource additions, and use [infrastructure configuration](/docs/platform/infrastructure/configuration) to customize defaults per environment.

## Preview Environments

[Preview environments](/docs/platform/deploy/preview-environments) are perfect for testing AI-generated changes. Each pull request gets its own isolated environment with real infrastructure.

This means you can:

- Let AI generate features and immediately test them with real databases and services
- Review AI-generated infrastructure changes before they hit production
- Catch issues early in isolated environments

## Production Observability

Encore Cloud provides [distributed tracing](/docs/platform/observability/tracing) and [metrics](/docs/platform/observability/metrics) across all your environments. You can:

- Analyze traces to debug issues across services
- Inspect timing to find bottlenecks
- Compare behavior between preview and production environments

## Connecting Your Cloud

To deploy your Encore app to your cloud:

1. [Sign up for Encore Cloud](https://app.encore.dev)
2. [Connect your AWS or GCP account](/docs/platform/deploy/own-cloud)
3. Push your Encore app to deploy

Encore Cloud provisions infrastructure in your cloud account based on the primitives declared in your code. You maintain full control and ownership of your infrastructure.

## What AI Can Provision

With Encore Cloud connected, AI-generated code can provision:

| Resource | AWS | GCP |
|----------|-----|-----|
| Databases | RDS (PostgreSQL) | Cloud SQL |
| Pub/Sub | SNS + SQS | Cloud Pub/Sub |
| Object Storage | S3 | Cloud Storage |
| Cron Jobs | CloudWatch Events | Cloud Scheduler |
| Secrets | Secrets Manager | Secret Manager |
| Caching | ElastiCache | Memorystore |

All resources are provisioned with security best practices like least-privilege IAM policies, private networking, and encryption. See the [AWS](/docs/platform/infrastructure/aws) and [GCP](/docs/platform/infrastructure/gcp) infrastructure docs for specifics.

## Learn More

- [Connect Your Cloud Account](/docs/platform/deploy/own-cloud)
- [Preview Environments](/docs/platform/deploy/preview-environments)
- [Infrastructure Configuration](/docs/platform/infrastructure/configuration)
- [Framework AI Integration (TypeScript)](/docs/ts/ai-integration)
- [Framework AI Integration (Go)](/docs/go/ai-integration)
