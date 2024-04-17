---
seotitle: Cloud Infrastructure Provisioning
seodesc: Learn how to provision appropriate cloud infrastructure depending on the environment type for AWS and GCP.
title: Infrastructure provisioning
subtitle: How Encore provisions infrastructure for your application
---

Encore automatically provisions all necessary infrastructure, in all environments and across all major cloud providers, without requiring application code changes. You simply [connect your cloud account](./own-cloud) and create an environment.

<img src="/assets/docs/encore_overview.png" title="Infrastructure Overview" className="noshadow"/>

This is powered by Encore's [Infrastructure SDK](/docs/primitives/overview), which lets you declare infrastructure resources (databases, caches, queues, scheduled jobs, etc.) as type-safe objects in application code.

At compile time, Encore creates an [Application Model](/docs/introduction#meet-the-encore-application-model) with a definition of the infrastructure your application requires. Encore then uses this model to provision the infrastructure in both your cloud account, and development and preview environments in Encore Cloud.

The approach removes the need for infrastructure configuration files and avoids creating cloud-specific dependencies in your application.

Having an end-to-end integration between application code and infrastructure also enables Encore to keep environments in sync and track cloud infrastructure, giving you an up-to-date view of your infrastructure to avoid unnecessary cloud costs.

<img src="/assets/docs/infra_config_new.png" title="Infrastructure Tracking"/>

## Environment types

By default, Encore provisions infrastructure using contextually appropriate objectives for each environment type. You retain control over the infrastructure in your cloud account, and can configure it directly both via Encore's Cloud Dashboard and your cloud provider's console. Encore takes care of syncing your changes.

|                        | Local              | Encore Cloud               | GCP / AWS                          |
| ---------------------- | ------------------ | -------------------------- | ---------------------------------- |
| **Environment types:** | Development        | Preview, Development       | Development, Production            |
| **Objectives:**        | Provisioning speed | Provisioning speed, Cost\* | Reliability, Security, Scalability |

\*Encore Cloud is free to use, subject to Fair Use guidelines and usage limits. [Learn more](/docs/about/usage)

## Development Infrastructure

Encore provisions infrastructure resources differently for each type of development environment.

|                    | Local                             | Preview / Development (Encore Cloud)         | GCP / AWS                                                      |
| ------------------ | --------------------------------- | -------------------------------------------- | -------------------------------------------------------------- |
| **SQL Databases:** | Docker                            | Encore Managed (Kubernetes)                  | [See production](/docs/deploy/infra#production-infrastructure) |
| **Pub/Sub:**       | In-memory ([NSQ](https://nsq.io)) | GCP Pub/Sub                                  | [See production](/docs/deploy/infra#production-infrastructure) |
| **Caches:**        | In-memory (Redis)                 | In-memory (Redis)                            | [See production](/docs/deploy/infra#production-infrastructure) |
| **Cron Jobs:**     | Disabled                          | [Encore Managed](/docs/primitives/cron-jobs) | [See production](/docs/deploy/infra#production-infrastructure) |

### Local Development

For local development Encore provisions a combination of Docker and in-memory infrastructure components.
[SQL Databases](/docs/primitives/databases) are provisioned using [Docker](https://docker.com). For [Pub/Sub](/docs/primitives/pubsub)
and [Caching](/docs/primitives/caching) the infrastructure is run in-memory. 

When running tests, a separate SQL Database cluster is provisioned that is optimized for high performance
(using an in-memory filesystem and fsync disabled) at the expense of reduced reliability.

To avoid surprises during development, [Cron Jobs](/docs/primitives/cron-jobs) are not triggered in local environments.
They can always be triggered manually by calling the API directly from the [development dashboard](/docs/observability/dev-dash).

The application code itself is compiled and run natively on your machine (without Docker).

### Preview Environments

When you've [connected your application to GitHub](/docs/how-to/github), Encore automatically provisions a temporary [Preview Environment](/docs/deploy/preview-environments) for each Pull Request.

Preview Environments are created in Encore Cloud, and are optimized for provisioning speed and cost-effectiveness.
The Preview Environment is automatically destroyed when the Pull Request is merged or closed.

Preview Environments are named after the pull request, so PR #72 will create an environment named `pr:72`.

### Encore Cloud

Encore Cloud is a simple, zero-configuration hosting solution provided by Encore.
It's perfect for development environments and small-scale use that do not require any specific SLAs.
It's also a great way to evaluate Encore without needing to connect your cloud account.

Encore Cloud is not designed for business-critical use and does not offer reliability guarantees for persistent storage
like SQL Databases. Other infrastructure primitives like [Pub/Sub](/docs/primitives/pubsub) and [Caching](/docs/primitives/caching)
are provisioned with small-scale use in mind.

[Learn more about the usage limitations](/docs/about/usage)

## Production Infrastructure

Encore provisions production infrastructure resources using best-practice guidelines and services for each respective cloud provider.

|                    | GCP                                                                        | AWS                                                                      |
| ------------------ | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| **Networking:**    | [VPC](#google-cloud-platform-gcp)                                          | [VPC](#amazon-web-services-aws)                                          |
| **Compute:**       | [Cloud Run](#google-cloud-platform-gcp), [GKE](#google-cloud-platform-gcp) | [Fargate ECS](#amazon-web-services-aws), [EKS](#amazon-web-services-aws) |
| **SQL Databases:** | [GCP Cloud SQL](#sql-databases)                                            | [Amazon RDS](#sql-databases-1)                                           |
| **Pub/Sub:**       | [GCP Pub/Sub](#pubsub)                                                     | [Amazon SQS][aws-sqs] & [Amazon SNS](#pubsub-1)                          |
| **Caches:**        | [GCP Memorystore (Redis)](#caching)                                        | [Amazon ElastiCache (Redis)](#caching-1)                                 |
| **Cron Jobs:**     | [Encore Managed](/docs/primitives/cron-jobs)                               | [Encore Managed](/docs/primitives/cron-jobs)                             | [Encore Managed](/docs/primitives/cron-jobs) |
| **Secrets:**       | [Secret Manager][gcp-secrets]                                              | [AWS Secrets Manager][aws-secrets]                                       |

### Configurability

With Encore you do not define any cloud service specifics in the application code. This means that after deploying, you can safely use your cloud provider's console to modify the provisioned resources, or use the built-in configuration UI in Encore's Cloud Dashboard. Encore takes care of syncing the changes automatically in both directions.

In the future, Encore will offer automated optimization of cloud environments according to your application's real-world behavior.

<img src="/assets/docs/infra_config.png" title="Infra configuration UI"/>

#### Process allocation

You can configure how microservices should be deployed on the compute hardware; either deploying all services in one process or one process per service.

It's often recommended to deploy all services in one process in order to reduce costs and minimize response times between services. (But it depends on your use case.)
Deploying each service as its own process will improve scalability and decrease blast radius if things go wrong. This is only recommended for production environments.

<img src="/assets/docs/microservices-process-allocation.png" title="Process allocation config"/>

### Google Cloud Platform (GCP)

[gcp-vpc]: https://cloud.google.com/vpc
[gcp-cloudrun]: https://cloud.google.com/run
[gcp-gke]: https://cloud.google.com/kubernetes-engine
[gcp-secrets]: https://cloud.google.com/secret-manager
[gcp-pubsub]: https://cloud.google.com/pubsub
[gcp-cloudsql]: https://cloud.google.com/sql
[gcp-redis]: https://cloud.google.com/memorystore

Encore provisions a single GCP Project for each environment, containing a single [Virtual Private Cloud (VPC)][gcp-vpc], and a whole slew of miscellaneous resources (IAM roles, policies, subnets, security groups, route tables, and so on). Secrets are stored using [Secret Manager][gcp-secrets].

#### Compute instances

When using GCP you can decide between [Cloud Run][gcp-cloudrun] (a fully managed infrastructure that scales to zero) or a [Google Kubernetes Engine][gcp-gke] (GKE) cluster.
If you prefer you can also [import an existing Kubernetes cluster](/docs/how-to/import-kubernetes-cluster) and have Encore deploy to it.

#### SQL Databases
When using [SQL Databases](/docs/primitives/databases), Encore provisions a single [GCP Cloud SQL][gcp-cloudsql] cluster, and separate databases within that cluster. The cluster is configured with the latest PostgreSQL version available at the time of provisioning.

The machine type is chosen as the smallest available that supports auto-scaling (1 vCPU / 3.75GiB memory).
You can freely increase the machine type yourself to handle larger scales.

Additionally, Encore sets up:
* Automatic daily backups (retained for 7 days) with point-in-time recovery
* Private networking, ensuring the database is only accessible from the VPC
* Mutual TLS encryption for additional security
* High availability mode with automatic failover (via disk replication to multiple zones)

#### Pub/Sub
When using [Pub/Sub](/docs/primitives/pubsub), Encore provisions [GCP Pub/Sub][gcp-pubsub] topics and subscriptions. Additionally, Encore automatically creates and configures dead-letter topics.

#### Caching
When using [Caching](/docs/primitives/caching), Encore provisions [GCP Memorystore for Redis][gcp-redis] clusters.

The machine type is chosen as the smallest available that supports auto-scaling (5GiB memory, with one read replica).
You can freely change the machine type yourself to handle larger scales.

Additionally, Encore sets up:
* Redis authentication
* Transit encryption with TLS for additional security
* A 10% memory buffer to better memory fragmentation, and active defragmentation

#### Cron Jobs
When using [Cron Jobs](/docs/primitives/cron-jobs), Encore Cloud triggers the execution
of cron jobs by calling the corresponding API using a signed request so the application can verify
the source of the request as coming from Encore's cron functionality. No infrastructure is
provisioned for this to work.

### Amazon Web Services (AWS)

[aws-vpc]: https://docs.aws.amazon.com/vpc/latest/userguide/what-is-amazon-vpc.html
[aws-fargate]: https://aws.amazon.com/fargate/
[aws-eks]: https://aws.amazon.com/eks/
[aws-secrets]: https://aws.amazon.com/secrets-manager/
[aws-rds]: https://aws.amazon.com/rds/postgresql/
[aws-sqs]: https://aws.amazon.com/sqs/
[aws-sns]: https://aws.amazon.com/sns/
[aws-redis]: https://aws.amazon.com/elasticache/redis/
[aws-ecr]: https://aws.amazon.com/ecr/

Encore provisions a dedicated [Virtual Private Cloud (VPC)][aws-vpc] for each environment. It contains an [Elastic Container Registry][aws-ecr] to host Docker images,
and a whole slew of miscellaneous resources (IAM roles, policies, subnets, security groups, route tables, and so on). Secrets are stored using [Secrets Manager][aws-secrets].

#### Compute instances

Encore provisions a [Fargate ECS][aws-fargate] cluster (managed, serverless, pay-as-you-go compute engine) or an [Elastic Kubernetes Service][aws-eks] (EKS) cluster.

#### SQL Databases
When using [SQL Databases](/docs/primitives/databases), Encore provisions a single [Amazon RDS][aws-rds] cluster, and separate databases within that cluster.
The cluster is configured with the latest PostgreSQL version available at the time of provisioning.

The instance type is chosen as the smallest available latest-generation type that supports auto-scaling (currently `db.m5.large`, with 2 vCPU / 8GiB memory).
You can freely change the instance type yourself to handle larger scales.

Additionally, Encore sets up:
* Automatic daily backups (retained for 7 days) with point-in-time recovery
* Private networking, ensuring the database is only accessible from the VPC
* Dedicated subnets for the database instances, with security group rules to secure them

#### Pub/Sub
When using [Pub/Sub](/docs/primitives/pubsub), Encore provisions a combination of [Amazon SQS][aws-sqs] and [Amazon SNS][aws-sns] topics and subscriptions.
Additionally, Encore automatically creates and configures dead-letter topics.

#### Caching
When using [Caching](/docs/primitives/caching), Encore provisions [Amazon ElastiCache for Redis][aws-redis] clusters.

The machine type is chosen as the smallest available that supports auto-scaling (currently `cache.m6g.large`, with one read replica).
You can freely change the machine type yourself to handle larger scales.

Additionally, Encore sets up:
* Redis ACL authentication
* A replication group, with multi-AZ replication and automatic failover for high availability
* Transit encryption with TLS for additional security
* A 10% memory buffer to better memory fragmentation, and active defragmentation

#### Cron Jobs
When using [Cron Jobs](/docs/primitives/cron-jobs), Encore Cloud triggers the execution
of cron jobs by calling the corresponding API using a signed request so the application can verify
the source of the request as coming from Encore's cron functionality. No infrastructure is
provisioned for this to work.
