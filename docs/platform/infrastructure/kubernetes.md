---
seotitle: How to deploy your Encore application to a new Kubernetes cluster
seodesc: Learn how to automatically deploy your Encore application to a new Kubernetes cluster.
title: Kubernetes deployment
subtitle: Deploying your app to a new Kubernetes cluster
lang: platform
---

# Deploying Encore Apps to Kubernetes

Encore Cloud gives you flexibility in where you run your applications. You have two options for Kubernetes deployments:

1. **Deploy to a new cluster**: Encore Cloud can automatically provision and manage a new Kubernetes cluster in your cloud account on AWS or GCP.
2. **Use an existing cluster**: Deploy to your pre-existing Kubernetes cluster ([see instructions here](/docs/platform/infrastructure/import-kubernetes-cluster))

All infrastructure provisioning is automated, and configuration is managed through the [Encore Cloud Dashboard](https://app.encore.cloud), keeping your application code clean and infrastructure-agnostic.

## Deploying to a new Kubernetes cluster

**1. Connect your cloud account:** Ensure your cloud account (Google Cloud Platform or AWS) is connected to Encore Cloud. ([See docs](/docs/platform/deploy/own-cloud))

**2. Create environment:** Open your app in the [Encore Cloud dashboard](https://app.encore.cloud) and go to **Environments**, then click on **Create Environment**.

Next, select your cloud (AWS or GCP) and then specify Kubernetes as the compute platform. Encore Cloud supports deploying to GKE on GCP, and EKS Fargate on AWS.

You can also configure if you want to allocate all services in one process or run one process per service.

<img src="/assets/docs/k8s-config.jpg" title="Environment Settings" className="mx-auto"/>

**3. Push your code:** To deploy, commit and push your code to the branch you configured as the deployment trigger. You can also trigger a manual deploy from the Cloud Dashboard by going to the **Environment Overview** page and clicking on **Deploy**.

**4. Automatic deployment by Encore Cloud:** Once you've triggered the deploy, Encore Cloud will automatically provision and deploy the necessary infrastructure on Kubernetes, per your environment configuration in the Cloud Dashboard. You can monitor the status of your deploy and view your environment's details through the Encore Cloud Dashboard.

**5. Accessing your cluster with kubectl:** You can access your cluster using the `kubectl` CLI tool. [See the docs](/docs/platform/infrastructure/configure-kubectl) for how to do this.

## Infrastructure Overview

Encore Cloud simplifies the process of deploying applications by automatically provisioning and managing the necessary Kubernetes components. Here's an overview of the components Encore Cloud manages and how they work together to support your applications.

### Namespace Management

Encore Cloud creates a unique namespace for each environment deployed to your Kubernetes cluster, ensuring complete isolation between different environments of your application.

### Secrets Management

Encore Cloud provides comprehensive secrets management through deep integration with Kubernetes Secrets. Application secrets that you configure in Encore Cloud are automatically stored as Kubernetes Secrets and made available to your services at runtime. This includes both application-specific secrets that you define, as well as infrastructure secrets like database credentials that Encore Cloud manages automatically.

Service accounts are automatically bound to the appropriate secrets they need access to, ensuring each service can only access the secrets it requires. This follows the principle of least privilege and helps maintain a strong security posture.

### Ingress Configuration

Encore Cloud provisions and manages ingress for your applications through a cloud provider-specific ingress controller. The ingress controller is automatically configured to handle incoming traffic and route it securely to your application's Encore Gateway service. It manages TLS certificates automatically to ensure all traffic is encrypted, and provides fine-grained control over which services are accessible from the public internet. The controller configuration is optimized for your specific cloud provider to ensure the best possible performance and reliability.

## Service Management

### Deployments
Encore Cloud manages the deployment configuration for each service in your application. Each service is deployed as a separate Kubernetes deployment, allowing for independent scaling and management. The deployment configurations are automatically generated and optimized based on your service's requirements.

For each service, Encore Cloud configures the pod specifications with appropriate resource requests and limits, health checks, and container settings. Runtime configurations like environment variables and command arguments are automatically set based on your application's needs. The container orchestration is handled seamlessly, with Encore Cloud managing pod scheduling, updates, and scaling to ensure your services run reliably and efficiently.

### Network Configuration

Encore Cloud provides a comprehensive networking setup through Kubernetes Service resources. Each service in your application gets assigned a unique cluster IP address, enabling reliable internal communication between services. This IP allocation works in conjunction with Kubernetes' built-in service discovery mechanism, allowing services to locate and communicate with each other using consistent internal DNS names. The internal service routing ensures that requests are efficiently distributed across all available pods for each service, providing automatic load balancing and failover capabilities.

### Identity and Access

Encore Cloud provides comprehensive service identity management through Kubernetes service accounts. Each pod is assigned its own dedicated service account, which handles authentication with the Kubernetes API and enables secure access to resources. These service accounts are automatically bound to the specific secrets and permissions required by each service.

For cloud provider integration, Encore Cloud maps the service accounts to appropriate IAM roles, enabling secure access to cloud resources like databases and object storage. Following the principle of least privilege, Encore Cloud configures the minimum required permissions for each service account, ensuring services can only access the resources they explicitly need.

All these configurations are automatically maintained and updated by Encore Cloud as you develop your application, ensuring your infrastructure stays aligned with your application's needs.