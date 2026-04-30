---
seotitle: Managing Infrastructure – Safety, Upgrades & Disaster Recovery
seodesc: Learn how Encore Cloud protects your infrastructure with deletion safeguards, handles upgrades, and supports disaster recovery for production environments.
title: Managing Infrastructure
subtitle: Deletion protection, upgrades, disaster recovery, and shared responsibilities
lang: platform
---

Encore Cloud provides built-in safeguards to protect your production infrastructure, manages upgrades across the stack, and gives you the tools to implement disaster recovery. This page covers the operational aspects of running production infrastructure with Encore Cloud.

## Infrastructure Safety

### Deletion protection

Encore Cloud implements multiple layers of protection against accidental or malicious deletion of production resources:

- **Admin-only environment deletion:** Only users with the Admin role can destroy or delete environments. [Learn more about roles](/docs/platform/management/permissions).
- **Confirmation required:** Destroying an environment requires the admin to manually type a confirmation message.
- **Stateful resource protection:** Stateful resources (databases, buckets, queues) are never deleted unless they are unused by the application and a user has manually confirmed deletion. Encore tracks which resources are in use through its [Application Model](/docs/ts/concepts/application-model).
- **Cloud provider safeguards:** For deployments to your own cloud, SQL databases have deletion protection enabled at the cloud provider level, which must be manually disabled in the cloud provider console before the resource can be removed.

### Drift detection

Encore Cloud is designed to coexist with manual changes made in your cloud provider's console. When deploying, Encore pulls the current resource properties before making any changes. If drift is detected (i.e. the resource was modified outside of Encore), Encore updates its internal representation to match the current state, unless there is a pending, manually requested property change in the Encore Cloud dashboard that hasn't been applied yet.

This means you can safely make changes directly in your cloud provider's console without worrying about Encore overwriting them. Learn more in the [Infrastructure Configuration](/docs/platform/infrastructure/configuration) docs.

### Audit trail

Encore Cloud maintains a versioned infrastructure graph where each node and edge is an immutable, versioned entity. Changes are applied by adding a new version, creating a complete audit trail:

- Each infrastructure graph version is linked to the deployment that applied it.
- Each deployment is connected to a specific git commit and any manually requested infrastructure changes.
- Each infrastructure change request is linked to the user who requested it.
- If a deployment was triggered manually (rather than via git push), the triggering user is also recorded.

This gives you full traceability from any infrastructure state back to the code change, change request, and user responsible.

## Deployment Phases

When Encore Cloud deploys your application, it follows a three-phase process:

1. **Infrastructure provisioning:** Encore sets up any new infrastructure required by your code changes, such as databases, IAM policies, Pub/Sub topics, and other resources.
2. **Deployment:** Encore deploys the new container images to your compute platform (Cloud Run, Fargate, Kubernetes, etc.).
3. **Cleanup:** Encore removes any unused stateless resources. Stateful resources are only removed if they are unused and deletion has been explicitly confirmed by a user.

### Deploy approval

For production environments, you can require admin approval before any deployment that includes infrastructure changes. When enabled, an Admin must manually review and approve the changes before the deployment proceeds. This includes IAM changes.

To configure this, go to **Encore Cloud dashboard > (Select your environment) > Settings > Infrastructure Approval**. Learn more in the [Environments](/docs/platform/deploy/environments) docs.

## Control Plane Separation

Encore Cloud separates the control plane from the runtime of your services. Your application runs independently in your own cloud environment, so if Encore Cloud's control plane is unavailable, your production services continue to operate normally.

The main impact of a control plane outage would be on control plane features like deployments, observability, and the Encore Cloud dashboard. Your running services, databases, and other infrastructure remain unaffected.

For recovery independently of Encore's service, you retain full ownership of your infrastructure and code. You can use the [open source tools](/docs/ts/self-host/build) to build images and deploy using any alternative CI/CD tooling.

## Upgrades

### Runtime and SDK upgrades

Encore runtime versions are automatically included as part of each build in the CI/CD pipeline. When you deploy, Encore builds your application with the latest runtime.

- **Backward compatibility:** The Encore SDK maintains backward compatibility. Breaking changes have never been introduced, and if one were necessary, it would be communicated ahead of time through release notes and directly to all customers on paid plans.
- **Custom runtime versions:** You can customize the Docker base image used for deployments through the `build.docker.base_image` setting in your `encore.app` file. This lets you pin a specific runtime version if needed. Learn more in the [Deploying](/docs/platform/deploy/deploying) docs.
- **Rollback:** To roll back a runtime version, update the configuration to specify the desired version and re-deploy.
- **Testing upgrades:** While generally no action is required due to backward compatibility, you can verify behavior by deploying to a staging/development environment first.

### Database upgrades (PostgreSQL)

Encore Cloud automatically applies schema migrations as part of each deployment. However, PostgreSQL major version upgrades (e.g. PostgreSQL 15 to 16) are handled differently:

- **Major version upgrades** must be manually initiated. They involve downtime and should be planned accordingly. Expect 10 to 20 minutes of downtime depending on database size, controlled by the cloud provider.
- **Migration tooling:** Standard cloud provider tools and PostgreSQL utilities like `pg_dump`/`pg_restore` are supported for migrating data between database versions or environments.
- **Testing:** Always test major upgrades in a staging environment before rolling out to production. Major PostgreSQL version upgrades may introduce backward compatibility issues at the SQL level.

### Infrastructure component upgrades

Upgrades to the underlying cloud services (Cloud Run, Pub/Sub, IAM, VPC, etc.) are handled as part of the normal [deployment phases](#deployment-phases). By default, these upgrades are applied automatically during deployment. You can require admin approval for deployments that include infrastructure changes through the [deploy approval](#deploy-approval) settings.

## Disaster Recovery

### Built-in protections

When deploying to your own cloud account, Encore Cloud provisions databases with the following protections by default:

- **Automated daily backups** retained for 7 days
- **Point-in-time recovery (PITR)** capabilities
- **Private subnet placement** for network isolation

Learn more about the default database configuration in the [GCP Infrastructure](/docs/platform/infrastructure/gcp) and [AWS Infrastructure](/docs/platform/infrastructure/aws) docs.

### Configuring DR

Disaster recovery settings for stateful resources can be configured in two ways:

1. **Encore Cloud dashboard:** Use the infrastructure configuration UI to adjust backup schedules, retention periods, and other DR-related settings.
2. **Cloud provider console:** Since Encore Cloud deploys infrastructure directly into your cloud account, you can configure DR settings (high availability, multi-zone, cross-region replication, etc.) directly in your cloud provider's console.

Learn more about manual configuration in the [Infrastructure Configuration](/docs/platform/infrastructure/configuration) docs.

### Recommended practices

For production environments with specific RTO/RPO targets, consider:

- Configuring automated backups with appropriate retention periods
- Enabling point-in-time recovery for databases
- Setting up high-availability and multi-zone configurations
- Testing recovery procedures regularly in non-production environments
- Documenting your recovery runbooks

## Shared Responsibility Model

Encore Cloud and your team share responsibility for operating production infrastructure:

| Area | Encore Cloud | Customer |
| --- | --- | --- |
| **Infrastructure provisioning** | Automatically provisions and manages cloud resources based on your application code | Defines infrastructure requirements through application code |
| **Deployments** | Builds, tests, and deploys automatically via CI/CD | Pushes code changes; optionally reviews and approves deployments |
| **Schema migrations** | Automatically applies database migrations on deploy | Writes and tests migration files |
| **Runtime upgrades** | Includes latest runtime in each build | Optionally pins specific versions; tests in staging |
| **PostgreSQL major upgrades** | N/A (must be manually initiated) | Plans and executes major version upgrades; tests in staging first |
| **Infrastructure configuration** | Provides defaults following best practices | Customizes settings via dashboard or cloud console as needed |
| **Disaster recovery** | Provides default backups and PITR for databases | Configures DR settings to meet specific RTO/RPO requirements |
| **Security & IAM** | Automatically manages IAM with least-privilege; encrypts data in transit and at rest | Reviews permissions; manages application-level auth |
| **Monitoring & alerting** | Provides built-in observability (tracing, metrics, logs) | Sets up custom alerts and monitors for application-specific needs |
| **Cloud provider updates** | Manages GCP/AWS deprecations as they arise | Stays informed of cloud provider deprecation notices |
