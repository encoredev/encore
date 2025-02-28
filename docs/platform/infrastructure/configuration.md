---
seotitle: Infrastructure Configuration
seodesc: Learn how you can configure infrastructure provisioned using Encore Cloud
title: Infrastructure Configuration
subtitle: How to configure infrastructure when using Encore Cloud
lang: platform
---

Encore Cloud provides a powerful and flexible approach to infrastructure management, ensuring that your cloud resources are efficiently provisioned, according to enterprise best practices.

Unlike traditional Infrastructure-as-Code (IaC) tools, when using Encore's declarative infrastructure framework, you do not define any cloud service specifics in code. This ensures your code is cloud-agnostic and portable across clouds, and can be deployed using different infrastructure for each environment according to your priorities (cost, performance, etc.).

Infrastructure configuration is made in the Encore Cloud dashboard, which provides a controlled workflow, role-based access controls, and auditable history of changes.

Encore Cloud provisions and manages infrastructure by using your cloud provider's APIs. Learn more in the [Infrastructure](/docs/platform/infrastructure/infra) documentation.

## Infrastructure settings when creating a new environment

When creating a new environment, you can decide the following:

- Which cloud provider to use (AWS or GCP)
- Which compute hardware to use (e.g. AWS Fargate, GCP Cloud Run, Kubernetes)
- If using Kubernetes, should a new cluster be created or should an existing cluster be used?
- Which Kubernetes provider to use (GKE or EKS)
- Which database to use (e.g. AWS RDS, GCP CloudSQL, Neon Serverless Postgres)
- Which process allocation strategy to use (more on this below)

## Ongoing infrastructure configuration

### Configuration UI in Encore Cloud

After creating an environment, you can continue to configure the infrastructure via the Encore Cloud dashboard.

The dashboard exposes the most common configuration options, and provides a controlled workflow for making changes, including audit logs and role-based access controls.

<img src="/assets/docs/infra_config.png" title="Infra configuration UI"/>

#### Process allocation configuration

Encore provides a powerful configuration option called process allocation. This enables you to configure how microservices should be deployed on the compute hardware; either deploying all services in one process or one process per service. All without any code changes.

It's often recommended to deploy all services in one process in order to reduce costs and minimize response times between services. (But it depends on your use case.)
Deploying each service as its own process will improve scalability and decrease blast radius if things go wrong. This is only recommended for production environments.

<img src="/assets/docs/microservices-process-allocation.png" title="Process allocation config"/>

### Manual configuration in your cloud provider's console

Manual configuration is relevant in cases where some configuration options are not yet available in the Encore Cloud dashboard, or you may want to make changes manually. Handily, you have full access to make changes directly in your cloud provider's console.

Encore Cloud tries very hard to ensure that any manual changes made in the cloud provider's console are not overwritten.

Therefore it only makes the minimum necessary modifications to infrastructure when deploying new changes, using the following strategies:

- **PATCH-style updates:** Resources are updated using compare-and-set and similar techniques, modifying only the attributes that require changes.

- **Avoid full syncs:** Unlike Terraform, Encore Cloud updates only the specific resources necessary to accomplish an infrastructure change rather than performing a complete infrastructure refresh.

These behaviors ensure an efficient and predictable workflow, minimizing unintended changes and reducing deployment times, and means that you can safely use your cloud provider's console to modify the provisioned resources.

This behavior also makes Encore Cloud well-suited for environments where infrastructure is partially managed outside of Encore Cloud, enabling you to deploy Encore applications alongside existing infrastructure (more on this below).

## Working with Existing Infrastructure

One of Encore Cloud’s strengths is its ability to work seamlessly with existing infrastructure. Since it does not enforce a full sync approach, it can:

- Integrate with pre-existing cloud resources without overwriting manual changes

- Deploy to existing Kubernetes clusters

- Co-exist with other IaC tools like Terraform and CloudFormation.

Encore Cloud also provides a Terraform Provider to simplify integration with existing Terraform-managed infrastructure. Learn more in the [Terraform Provider](/docs/platform/integrations/terraform) documentation.
