---
seotitle: GCP Infrastructure on Encore Cloud
seodesc: A comprehensive guide to how Encore Cloud provisions and manages GCP infrastructure for your applications
title: GCP Infrastructure
subtitle: Understanding your application's GCP infrastructure
lang: platform
---

Encore Cloud simplifies the process of deploying applications by automatically provisioning and managing the necessary GCP infrastructure. This page provides an overview of the components involved and how they work together to support your applications.

## Core Infrastructure Components

### Networking Architecture

To ensure maximum security and isolation, Encore Cloud provisions a dedicated GCP Project for each environment. This project isolation prevents any potential cross-environment access and enables granular control over resources and permissions. Within each project, all resources are deployed into a private network configuration, where they can only communicate with other resources inside the VPC. This private networking approach significantly reduces the attack surface by preventing direct access from the public internet, with traffic only flowing through designated ingress points.

### Container Management

Encore Cloud provisions a [Google Container Registry (GCR)][gcp-gcr] to store your application's Docker images.

The registry implements comprehensive access controls to ensure only authorized users and services can access and manage container images. Through integration with GCP's Identity and Access Management (IAM), each service is granted the minimum required permissions needed to pull its container images.

Additionally, GCR performs automated vulnerability scanning on all container images. As new images are pushed to the registry, they are automatically analyzed for known security vulnerabilities in the operating system and application dependencies. This proactive scanning helps identify potential security issues early in the deployment pipeline, allowing you to maintain a secure application environment.

### Secrets Management

Encore Cloud's integration with Secret Manager provides comprehensive security and seamless access to sensitive data. All secrets are automatically injected as environment variables into your services, eliminating the need for manual configuration while maintaining security. The secrets are protected using industry-standard encryption both when stored and during transmission between services. To ensure maximum security, Secret Manager implements strict access controls - each service can only access the specific secrets it needs, and all access attempts are logged and audited.

## Compute Options

Encore Cloud provisions one of two compute platforms for running your application containers, based on your choice:

### Google Cloud Run

When using Cloud Run, Encore Cloud configures:

**Service Deployments**
  Each service is configured with optimized container settings and health check configurations to ensure reliable operation. Environment variables are automatically injected from Secret Manager to securely provide configuration values. Service discovery integration enables seamless communication between services.

**Cloud Run Services**
  Cloud Run services are configured with zero-downtime deployment strategies, ensuring your application remains available during updates. Each service is integrated with a load balancer to distribute traffic efficiently across instances. Health check grace periods are configured to allow containers adequate time to start up before receiving traffic, preventing premature termination of healthy instances.

**IAM Configuration**
  Each deployment receives its own dedicated service account to ensure proper isolation and security. These service accounts are automatically configured with the minimum required permissions needed for operation. This includes access to pull container images from Google Container Registry, write application logs to Cloud Logging, and interact with assigned GCP resources like Cloud Storage buckets and Pub/Sub topics. The service accounts are also granted permission to read secrets from Secret Manager, enabling secure access to sensitive configuration values. This automated permission management ensures your services have exactly the access they need while following security best practices.


### Google Kubernetes Engine

When using GKE, Encore Cloud configures:

- **Cluster Setup**
  Encore Cloud provisions either GKE Autopilot clusters or standard GKE clusters with managed node pools, both configured to run in private subnets for enhanced security. With Autopilot, GKE automatically manages the underlying infrastructure, while with standard clusters Encore Cloud configures and maintains optimized node pools based on your workload requirements. In both cases, the nodes are placed in private subnets to ensure they're not directly accessible from the internet, with all traffic flowing through the load balancer.

- **Kubernetes Resources**
  Encore Cloud automatically creates and manages all necessary Kubernetes resources for your application. Each Encore service is deployed as a Kubernetes Deployment, ensuring reliable operation and scaling capabilities. These deployments are backed by service accounts configured with appropriate IAM roles to access GCP resources securely. Sensitive configuration data is stored as Kubernetes Secrets and automatically mounted into the appropriate pods. To enable network connectivity, Encore Cloud provisions Kubernetes Service and Ingress resources that integrate with the Google Cloud Load Balancer, providing secure external access to your application endpoints.

- **Load Balancer Integration**
  Encore Cloud integrates with Google Cloud Load Balancer to provide secure and reliable access to your applications. The load balancer is configured to distribute traffic across your services while handling SSL/TLS termination. All traffic is automatically encrypted using managed SSL/TLS certificates that are provisioned and renewed automatically. This ensures your application endpoints remain secure and accessible through HTTPS without requiring manual certificate management.

- **Monitoring Setup**
  Encore Cloud sets up comprehensive monitoring for your GKE clusters by configuring both metrics collection and log management. Container metrics are automatically collected from each pod and exported to your configured monitoring service, providing detailed insights into resource usage, performance, and application behavior. Additionally, all container logs are seamlessly forwarded to Cloud Logging, enabling centralized log aggregation and analysis. This integrated monitoring approach gives you full visibility into your application's health and performance within the Google Cloud ecosystem.

- **Service Accounts**
  Encore Cloud implements a comprehensive service account management system that ensures secure and controlled access to GCP resources. Each service in your application receives its own dedicated service account, providing fine-grained access control and isolation between services.

  These service accounts are automatically configured with IAM roles that map precisely to the GCP services your application needs to interact with. The permission configuration is handled dynamically based on your application's declared resource usage. For example, if your service needs to access a GCS bucket, Encore Cloud automatically grants the minimum required permissions for those specific storage operations. Similarly, when your service needs to publish or subscribe to Pub/Sub topics, connect to databases, or retrieve secrets, the appropriate IAM roles are configured automatically.

  This automated permission management ensures that each service operates under the principle of least privilege, having access only to the resources it explicitly needs to function. This significantly enhances your application's security posture by minimizing the potential impact of any security breach.


All of these configurations are automatically maintained and updated by Encore Cloud as you develop your application, ensuring your infrastructure stays aligned with your application's needs.

## Managed Services

### Databases
Encore Cloud provisions [GCP Cloud SQL][gcp-cloudsql] for PostgreSQL databases, providing a robust and scalable database solution:

Encore Cloud provisions Cloud SQL instances running the latest PostgreSQL version, ensuring you have access to the newest features and security updates. Each instance starts with the smallest available configuration to optimize costs, while maintaining the ability to automatically scale up resources as your application's needs grow.

Data protection is a key priority, with automated daily backups retained for 7 days and point-in-time recovery capabilities. This allows you to restore your database to any moment within the retention period if needed.

Security is enforced through strategic placement of databases in private subnets, isolating them from direct internet access. Strict access controls ensure that only authorized services and users can connect to the database instances.

### Pub/Sub
Encore Cloud implements a robust messaging system using [GCP Pub/Sub][gcp-pubsub]. The system is designed with reliability and security in mind, automatically configuring dead-letter topics to capture and preserve any failed messages for later analysis and debugging. Each service in your application receives precisely scoped IAM permissions for publishing and consuming messages, ensuring secure communication between components while maintaining the principle of least privilege. Encore Cloud fully manages all subscriptions and topics, handling the complex setup and ongoing maintenance of your messaging infrastructure, allowing you to focus on your application logic rather than infrastructure management.

### Object Storage
Encore Cloud leverages [Google Cloud Storage][gcp-gcs] for object storage needs. When you declare storage buckets in your application, Encore Cloud automatically provisions them with unique names in GCP. Each service that interacts with storage is configured with precisely scoped permissions, ensuring secure access to only the buckets and operations it requires. For public buckets, Encore Cloud integrates with Cloud CDN to optimize content delivery, with each bucket accessible through a unique URL. This comprehensive setup provides secure, efficient, and easily manageable object storage capabilities for your application.

### Caching
Encore Cloud uses [GCP Memorystore for Redis][gcp-redis] to provide a high-performance caching solution. Each Redis instance starts with the smallest available configuration to optimize costs while maintaining the ability to automatically scale up resources as your application's caching needs grow. The instances are configured in a high-availability setup to ensure your cache remains available and performant even during infrastructure updates or zone outages. Access to the cache is secured through Redis authentication, with credentials automatically managed and rotated by Encore Cloud to maintain a strong security posture.

### Cron Jobs
Encore Cloud provides a streamlined approach to scheduled task execution that prioritizes both simplicity and security. Each cron job is executed through authenticated API requests that are cryptographically signed, ensuring that only legitimate, verified requests can trigger your scheduled tasks. The system includes robust source verification that validates all requests originate from Encore Cloud's trusted cron infrastructure. This elegant implementation requires no additional infrastructure components, making it both cost-effective and easy to maintain while providing the reliability and security needed for production workloads.

[gcp-vpc]: https://cloud.google.com/vpc
[gcp-cloudrun]: https://cloud.google.com/run
[gcp-gke]: https://cloud.google.com/kubernetes-engine
[gcp-secrets]: https://cloud.google.com/secret-manager
[gcp-pubsub]: https://cloud.google.com/pubsub
[gcp-gcs]: https://cloud.google.com/storage
[gcp-cloudsql]: https://cloud.google.com/sql
[gcp-redis]: https://cloud.google.com/memorystore
[gcp-gcr]: https://cloud.google.com/container-registry
