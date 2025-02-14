---
seotitle: Cloud Infrastructure Provisioning
seodesc: Learn how to provision appropriate cloud infrastructure depending on the environment type for AWS and GCP.
title: Infrastructure provisioning
subtitle: How Encore Cloud provisions infrastructure for your application
lang: platform
---

Encore Cloud automatically provisions all necessary infrastructure, in all environments and across all major cloud providers, without requiring application code changes. You simply [connect your cloud account](/docs/platform/deploy/own-cloud) and create an environment.

<img src="/assets/docs/encore_overview.png" title="Infrastructure Overview" className="noshadow"/>

This is powered by Encore's [Backend Framework](/docs/ts), which lets you declare infrastructure resources (databases, caches, queues, scheduled jobs, etc.) as type-safe objects in application code.

At compile time, Encore creates an [Application Model](/docs/ts/concepts/application-model) with a definition of the infrastructure your application requires. Encore Cloud then uses this model to provision the infrastructure in both your cloud account, and development and preview environments hosted by Encore Cloud.

The approach removes the need for infrastructure configuration files and avoids creating cloud-specific dependencies in your application.

Having an end-to-end integration between application code and infrastructure also enables Encore Cloud to keep environments in sync and track cloud infrastructure, giving you an up-to-date view of your infrastructure to avoid unnecessary cloud costs.

<img src="/assets/docs/infra_config_new.png" title="Infrastructure Tracking"/>

## Environment types

By default, Encore Cloud provisions infrastructure using contextually appropriate objectives for each environment type. You retain control over the infrastructure in your cloud account, and can configure it directly both via the Encore Cloud dashboard and your cloud provider's console. Encore Cloud takes care of syncing your changes.

|                        | Local              | Encore Cloud Hosting       | GCP / AWS                          |
| ---------------------- | ------------------ | -------------------------- | ---------------------------------- |
| **Environment types:** | Development        | Preview, Development       | Development, Production            |
| **Objectives:**        | Provisioning speed | Provisioning speed, Cost\* | Reliability, Security, Scalability |

\*Encore Cloud Hosting is free to use, subject to Fair Use guidelines and usage limits. [Learn more](/docs/platform/management/usage)

## Development Infrastructure

Encore Cloud provisions infrastructure resources differently for each type of development environment.

|                     | Local                             | Preview / Development (Encore Cloud Hosting)                 | GCP / AWS                                                      |
| ------------------- | --------------------------------- | ------------------------------------------------------------ | -------------------------------------------------------------- |
| **SQL Databases:**  | Docker                            | Encore Cloud Managed (Kubernetes), [Neon](/docs/deploy/neon) | [See production](/docs/deploy/infra#production-infrastructure) |
| **Pub/Sub:**        | In-memory ([NSQ](https://nsq.io)) | GCP Pub/Sub                                                  | [See production](/docs/deploy/infra#production-infrastructure) |
| **Caches:**         | In-memory (Redis)                 | In-memory (Redis)                                            | [See production](/docs/deploy/infra#production-infrastructure) |
| **Cron Jobs:**      | Disabled                          | [Encore Cloud Managed](/docs/primitives/cron-jobs)           | [See production](/docs/deploy/infra#production-infrastructure) |
| **Object Storage:** | Local Disk                        | Encore Cloud Managed                                         | [See production](/docs/deploy/infra#production-infrastructure) |


### Local Development

For local development Encore Cloud provisions a combination of Docker and in-memory infrastructure components.
SQL Databases are provisioned using [Docker](https://docker.com). For Pub/Sub
and Caching the infrastructure is run in-memory.

When running tests, a separate SQL Database cluster is provisioned that is optimized for high performance
(using an in-memory filesystem and fsync disabled) at the expense of reduced reliability.

To avoid surprises during development, Cron Jobs are not triggered in local environments.
They can always be triggered manually by calling the API directly from the [development dashboard](/docs/ts/observability/dev-dash).

The application code itself is compiled and run natively on your machine (without Docker).

### Preview Environments

When you've [connected your application to GitHub](/docs/platform/integrations/github), Encore Cloud automatically provisions a temporary [Preview Environment](/docs/platform/deploy/preview-environments) for each Pull Request.

Preview Environments are created in Encore Cloud Hosting, and are optimized for provisioning speed and cost-effectiveness.
The Preview Environment is automatically destroyed when the Pull Request is merged or closed.

Preview Environments are named after the pull request, so PR #72 will create an environment named `pr:72`.

### Encore Cloud Hosting

Encore Cloud Hosting is a simple, zero-configuration hosting solution provided by Encore.
It's perfect for development environments and small-scale use that do not require any specific SLAs.
It's also a great way to evaluate Encore Cloud without needing to connect your cloud account.

Encore Cloud Hosting is not designed for business-critical use and does not offer reliability guarantees for persistent storage
like SQL Databases. Other infrastructure primitives like Pub/Sub and Caching
are provisioned with small-scale use in mind.

[Learn more about the usage limitations](/docs/platform/management/usage)

## Production Infrastructure

Encore Cloud provisions production infrastructure resources using best-practice guidelines and services for each respective cloud provider.

|                     | GCP                                                                                                                                | AWS                                                                                                            |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| **Networking:**     | [VPC](/docs/platform/infrastructure/gcp#networking-architecture)                                                                   | [VPC](/docs/platform/infrastructure/aws#networking-architecture)                                               |
| **Compute:**        | [Cloud Run](/docs/platform/infrastructure/gcp#google-cloud-run), [GKE](/docs/platform/infrastructure/gcp#google-kubernetes-engine) | [Fargate ECS](/docs/platform/infrastructure/aws#aws-fargate), [EKS](/docs/platform/infrastructure/aws#aws-eks) |
| **SQL Databases:**  | [GCP Cloud SQL](/docs/platform/infrastructure/gcp#databases), [Neon](/docs/platform/infrastructure/neon)                           | [Amazon RDS](/docs/platform/infrastructure/aws#databases), [Neon](/docs/platform/infrastructure/neon)          |
| **Pub/Sub:**        | [GCP Pub/Sub](/docs/platform/infrastructure/gcp#pubsub)                                                                            | [Amazon SQS & Amazon SNS](/docs/platform/infrastructure/aws#pubsub)                                            |
| **Object Storage:** | [GCS/Cloud CDN](/docs/platform/infrastructure/gcp#object-storage)                                                                  | [Amazon S3/CloudFront](/docs/platform/infrastructure/aws#object-storage)                                       |
| **Caches:**         | [GCP Memorystore (Redis)](/docs/platform/infrastructure/gcp#caching)                                                               | [Amazon ElastiCache (Redis)](/docs/platform/infrastructure/aws#caching)                                        |
| **Cron Jobs:**      | Encore Cloud Managed                                                                                                               | Encore Cloud Managed                                                                                           | Encore Cloud Managed |
| **Secrets:**        | [Secret Manager](/docs/platform/infrastructure/gcp#secrets-management)                                                             | [AWS Secrets Manager](/docs/platform/infrastructure/aws#se)                                                    |

### Configurability

With Encore you do not define any cloud service specifics in the application code. This means that after deploying, you can safely use your cloud provider's console to modify the provisioned resources, or use the built-in configuration UI in the Encore Cloud dashboard. Encore Cloud takes care of syncing the changes automatically in both directions.

In the future, Encore Cloud will offer automated optimization of cloud environments according to your application's real-world behavior.

<img src="/assets/docs/infra_config.png" title="Infra configuration UI"/>

#### Process allocation

You can configure how microservices should be deployed on the compute hardware; either deploying all services in one process or one process per service.

It's often recommended to deploy all services in one process in order to reduce costs and minimize response times between services. (But it depends on your use case.)
Deploying each service as its own process will improve scalability and decrease blast radius if things go wrong. This is only recommended for production environments.

<img src="/assets/docs/microservices-process-allocation.png" title="Process allocation config"/>
