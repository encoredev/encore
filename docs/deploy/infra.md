---
title: Infrastructure
---

We developers are used to spending a large amount of our time managing infrastructure when we’d rather be working on the core logic. That’s where Encore is different.

Encore uses its understanding of your application, based on the [Encore Application Model](../application-model), to automatically provision, configure, and manage your infrastructure. Unlike other tools where you have to carefully describe what your backend infrastructure needs to look like, Encore figures this out on its own. Leaving you to focus on the enjoyable aspects of building a product: writing the core logic and iterating on solving problems.

### Deploy to your own cloud
Encore works seamlessly with all the major cloud providers (AWS, GCP, Azure). Deploying your application to your own cloud account is as easy as connecting the accounts together (precisely how differs per cloud provider), and then creating a new environment and selecting the cloud you want to deploy to. You can read more about this in the [Bring your own cloud](./own-cloud) docs.

## Infrastructure Setup
The precise infrastructure that Encore provisions depends on the cloud provider and the type of environment you select.

To run your application, Encore provisions a managed Kubernetes cluster for each environment (when deploying to your own cloud).

**Production Environments**

Encore sets up a VPC network and provisions all the resources within that VPC.

For databases, Encore will prompt you to provide instructions on the number of CPU cores, memory profile, and initial disk size. The databases are not exposed to the internet and can only be reached through the VPC. Backups are automatically configured.

The precise services used depends on the cloud provider: RDS for AWS, CloudSQL for GCP, and Azure Database for PostgreSQL for Azure.

**Development Environments**

For these environments, Encore optimizes for cost reduction instead and sets it up in as lightweight a way as possible. Databases are provisioned in Kubernetes, backed by a Persistent Disk, for a minimal footprint.


