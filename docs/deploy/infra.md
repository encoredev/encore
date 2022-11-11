---
seotitle: Cloud Infrastructure Provisioning
seodesc: Learn how to provision appropriate cloud infrastructure depending on the environment type, for all of the major cloud providers – AWS, GCP, and Azure.
title: Infrastructure provisioning
subtitle: How Encore provisions infrastructure for you
---

Encore automatically provisions all necessary infrastructure, in all environments and across all major cloud providers. All you need to do is [connect your cloud account](./own-cloud) and create an environment.

This is powered by the [Encore Application Model](/docs/introduction#meet-the-encore-application-model). It provides a precise definition of the infrastructure primitives each service requires. Encore uses this knowledge, combined with context about the environment type (production/development/preview) and the target cloud provider (GCP/AWS/Azure), to make informed decisions about how to provision the necessary infrastructure. Encore keeps all environments in sync, so you can be confident your changes will work when deployed to production.

|  | Encore Cloud | GCP / AWS / Azure |
| - | - | - | - | - |
| **Environment types:** | Preview, Development | Development, Production |
| **Objectives:** | Provisioning speed, Cost\* | Reliability, Security, Scalability |

\*Encore Cloud is free to use, subject to Fair Use guidelines and usage limits. [Learn more](/docs/about/usage)

## Configuration

With Encore you express cloud infrastructure as logical statements directly in your application code. After deploying to your own cloud account, you can safely use your cloud provider's console to modify the provisioned resources according to your application's scaling requirements. See more details below for each cloud provider and infrastructure resource.

## Development Infrastructure

Encore provisions infrastructure resources differently for each type of development environment.

|  | Local | Preview / Development (Encore Cloud) | GCP / AWS / Azure |
| - | - | - | - |
| **SQL Databases:** | Docker | Encore Managed (Kubernetes) | [See production](/docs/deploy/infra#production-infrastructure) |
| **Pub/Sub:** | In-memory ([NSQ](https://nsq.io)) | GCP Pub/Sub | [See production](/docs/deploy/infra#production-infrastructure) |
| **Caches:** | In-memory (Redis) | In-memory (Redis) | [See production](/docs/deploy/infra#production-infrastructure) |
| **Cron Jobs:** | Disabled | [Encore Managed][encore-cron] | [See production](/docs/deploy/infra#production-infrastructure) |

### Local Development

For local development Encore provisions a combination of Docker and in-memory infrastructure components.
[SQL Databases][encore-sqldb] are provisioned using [Docker](https://docker.com). For [Pub/Sub][encore-pubsub]
and [Caching][encore-caching] the infrastructure is run in-memory. 

When running tests, a separate SQL Database cluster is provisioned that is optimized for high performance
(using an in-memory filesystem and fsync disabled) at the expense of reduced reliability.

To avoid surprises during development, [Cron Jobs][encore-cron] are not triggered in local environments.
They can always be triggered manually by calling the API directly from the [development dashboard](/docs/observability/dev-dash).

The application code itself is compiled and run natively on your machine (without Docker).

### Preview Environments

When you've [connected your application to GitHub](/docs/how-to/github), Encore automatically provisions a temporary [Preview Environment](/docs/deploy/environments#preview-environments) for each Pull Request.

Preview Environments are created in Encore Cloud, and are optimized for provisioning speed and cost-effectiveness.
The Preview Environment is automatically destroyed when the Pull Request is merged or closed.

Preview Environments are named after the pull request, so PR #72 will create an environment named `pr:72`.

### Encore Cloud

Encore Cloud is a simple, zero-configuration hosting solution provided by Encore.
It's perfect for development environments and small-scale hobby use.
It's also a great way to evaluate Encore without having to connect your cloud account.

Encore Cloud is not designed for production use and does not offer reliability guarantees for persistent storage
like SQL Databases. Other infrastructure primitives like [Pub/Sub][encore-pubsub] and [Caching][encore-caching]
are provisioned with small-scale use in mind.

## Production Infrastructure

Encore provisions production infrastructure resources using best-practice guidelines and services for each respective cloud provider.

|  | GCP | AWS | Azure |
| - | - | - | - |
| **Networking:** | [VPC][gcp-vpc] | [VPC][aws-vpc] | [VPC][azure-vpc] |
| **Compute:** | [Cloud Run][gcp-cloudrun] | [Fargate ECS][aws-fargate] | [App Service][azure-app-service] & [App Service Plan][azure-app-service-plan] |
| **SQL Databases:** | [GCP Cloud SQL][gcp-cloudsql] | [Amazon RDS][aws-rds] | [Azure Database][azure-sqldb] |
| **Pub/Sub:** | [GCP Pub/Sub][gcp-pubsub] | [Amazon SQS][aws-sqs] & [Amazon SNS][aws-sns] | [Azure Service Bus][azure-service-bus] |
| **Caches:** | [GCP Memorystore (Redis)][gcp-redis] | [Amazon ElastiCache (Redis)][aws-redis] | [Azure Cache (Redis)][azure-redis] |
| **Cron Jobs:** | [Encore Managed][encore-cron] | [Encore Managed][encore-cron] | [Encore Managed][encore-cron] |
| **Secrets:** | [Secret Manager][gcp-secrets] | [AWS Secrets Manager][aws-secrets] | [App Service App][azure-app-service-secrets] |

### Google Cloud Platform (GCP)

[gcp-vpc]: https://cloud.google.com/vpc
[gcp-cloudrun]: https://cloud.google.com/run
[gcp-secrets]: https://cloud.google.com/secret-manager
[gcp-pubsub]: https://cloud.google.com/pubsub
[gcp-cloudsql]: https://cloud.google.com/sql
[gcp-redis]: https://cloud.google.com/memorystore

Encore provisions a single GCP Project for each environment, containing a single [Virtual Private Cloud (VPC)][gcp-vpc].
Within the VPC Encore provisions a [Cloud Run][gcp-cloudrun] service to run the application, storing secret values using [Secret Manager][gcp-secrets].

#### SQL Databases
When using [SQL Databases][encore-sqldb], Encore provisions a single [GCP Cloud SQL][gcp-cloudsql] cluster, and separate databases within that cluster. The cluster is configured with the latest PostgreSQL version available at the time of provisioning.

The machine type is chosen as the smallest available that supports auto-scaling (1 vCPU / 3.75GiB memory).
You can freely increase the machine type yourself to handle larger scales.

Additionally, Encore sets up:
* Automatic daily backups (retained for 7 days) with point-in-time recovery
* Private networking, ensuring the database is only accessible from the VPC
* Mutual TLS encryption for additional security
* High availability mode with automatic failover (via disk replication to multiple zones)

#### Pub/Sub
When using [Pub/Sub][encore-pubsub], Encore provisions [GCP Pub/Sub][gcp-pubsub] topics and subscriptions. Additionally, Encore automatically creates and configures dead-letter topics.

#### Caching
When using [Caching][encore-caching], Encore provisions [GCP Memorystore for Redis][gcp-redis] clusters.

The machine type is chosen as the smallest available that supports auto-scaling (5GiB memory, with one read replica).
You can freely change the machine type yourself to handle larger scales.

Additionally, Encore sets up:
* Redis authentication
* Transit encryption with TLS for additional security
* A 10% memory buffer to better memory fragmentation, and active defragmentation

#### Cron Jobs
When using [Cron Jobs][encore-cron], Encore's Cloud Platform triggers the execution
of cron jobs by calling the corresponding API using a signed request so the application can verify
the source of the request as coming from Encore's cron functionality. No infrastructure is
provisioned for this to work.

### Amazon Web Services (AWS)

[aws-vpc]: https://docs.aws.amazon.com/vpc/latest/userguide/what-is-amazon-vpc.html
[aws-fargate]: https://aws.amazon.com/fargate/
[aws-secrets]: https://aws.amazon.com/secrets-manager/
[aws-rds]: https://aws.amazon.com/rds/postgresql/
[aws-sqs]: https://aws.amazon.com/sqs/
[aws-sns]: https://aws.amazon.com/sns/
[aws-redis]: https://aws.amazon.com/elasticache/redis/
[aws-ecr]: https://aws.amazon.com/ecr/

Encore provisions a dedicated [Virtual Private Cloud (VPC)][aws-vpc] for each environment.
The VPC contains a [Fargate ECS][aws-fargate] cluster to run the application, an [Elastic Container Registry][aws-ecr] to host Docker images,
and a whole slew of miscellaneous resources (IAM roles, policies, subnets, security groups, route tables, and so on).
Secrets are stored using [Secrets Manager][aws-secrets].

#### SQL Databases
When using [SQL Databases][encore-sqldb], Encore provisions a single [Amazon RDS][aws-rds] cluster, and separate databases within that cluster.
The cluster is configured with the latest PostgreSQL version available at the time of provisioning.

The instance type is chosen as the smallest available latest-generation type that supports auto-scaling (currently `db.m5.large`, with 2 vCPU / 8GiB memory).
You can freely change the instance type yourself to handle larger scales.

Additionally, Encore sets up:
* Automatic daily backups (retained for 7 days) with point-in-time recovery
* Private networking, ensuring the database is only accessible from the VPC
* Dedicated subnets for the database instances, with security group rules to secure them

#### Pub/Sub
When using [Pub/Sub][encore-pubsub], Encore provisions a combination of [Amazon SQS][aws-sqs] and [Amazon SNS][aws-sns] topics and subscriptions.
Additionally, Encore automatically creates and configures dead-letter topics.

#### Caching
When using [Caching][encore-caching], Encore provisions [Amazon ElastiCache for Redis][aws-redis] clusters.

The machine type is chosen as the smallest available that supports auto-scaling (currently `cache.m6g.large`, with one read replica).
You can freely change the machine type yourself to handle larger scales.

Additionally, Encore sets up:
* Redis ACL authentication
* A replication group, with multi-AZ replication and automatic failover for high availability
* Transit encryption with TLS for additional security
* A 10% memory buffer to better memory fragmentation, and active defragmentation

#### Cron Jobs
When using [Cron Jobs][encore-cron], Encore's Cloud Platform triggers the execution
of cron jobs by calling the corresponding API using a signed request so the application can verify
the source of the request as coming from Encore's cron functionality. No infrastructure is
provisioned for this to work.

### Microsoft Azure

[azure-vpc]: https://azure.microsoft.com/en-us/products/virtual-network/
[azure-app-service]: https://azure.microsoft.com/en-us/products/app-service/
[azure-app-service-plan]: https://learn.microsoft.com/en-us/azure/app-service/overview-hosting-plans
[azure-app-service-secrets]: https://learn.microsoft.com/en-gb/azure/app-service/configure-common
[azure-sqldb]: https://azure.microsoft.com/en-gb/pricing/details/postgresql/flexible-server/
[azure-service-bus]: https://azure.microsoft.com/en-us/products/service-bus/
[azure-cr]: https://aws.amazon.com/secrets-manager/
[azure-redis]: https://azure.microsoft.com/en-gb/products/cache/
[azure-private-link]: https://learn.microsoft.com/en-us/azure/private-link/private-endpoint-overview

Encore provisions a dedicated [Virtual Private Cloud (VPC)][azure-vpc] for each environment,
containing an [App Service][azure-app-service] and [App Service Plan][azure-app-service-plan] to run the application.
Secrets are stored as part of the [App Service App][azure-app-service-secrets].

#### SQL Databases
When using [SQL Databases][encore-sqldb], Encore provisions a single [Azure Database for PostgreSQL][azure-sqldb] cluster, and separate databases within that cluster.
The cluster is configured with the latest PostgreSQL version available at the time of provisioning.

The instance type is chosen as the smallest available latest-generation type that supports auto-scaling (currently `D2s_v3`, with 2 vCPU / 8GiB memory).
You can freely change the instance type yourself to handle larger scales.

Additionally, Encore sets up:
* Automatic daily backups (retained for 7 days) with point-in-time recovery
* Private networking, ensuring the database is only accessible from the VPC
* Dedicated subnets for the database instances, with security group rules to secure them

#### Pub/Sub
When using [Pub/Sub][encore-pubsub], Encore provisions [Azure Service Bus][azure-service-bus] topics and subscriptions.
Additionally, Encore automatically creates and configures dead-letter topics.

#### Caching
When using [Caching][encore-caching], Encore provisions [Azure Cache for Redis][azure-redis] clusters.

The machine type is chosen as the smallest available that supports auto-scaling (currently `C1` with 1GiB memory).
You can freely change the machine type yourself to handle larger scales.

Additionally, Encore sets up:
* Redis authentication
* Transit encryption with TLS for additional security
* A 10% memory buffer to better memory fragmentation, and active defragmentation
* An [Azure Private Link][azure-private-link] connection for secure connectivity from the VPC

#### Cron Jobs
When using [Cron Jobs][encore-cron], Encore's Cloud Platform triggers the execution
of cron jobs by calling the corresponding API using a signed request so the application can verify
the source of the request as coming from Encore's cron functionality. No infrastructure is
provisioned for this to work.

[encore-sqldb]: /docs/develop/databases
[encore-pubsub]: /docs/develop/pubsub
[encore-caching]: /docs/develop/caching
[encore-cron]: /docs/develop/cron-jobs
