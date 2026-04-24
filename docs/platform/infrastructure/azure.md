---
seotitle: Azure Infrastructure on Encore Cloud
seodesc: A comprehensive guide to how Encore Cloud provisions and manages Azure infrastructure for your applications
title: Azure Infrastructure
subtitle: Understanding your application's Azure infrastructure
lang: platform
---

Encore Cloud simplifies the process of deploying applications by automatically provisioning and managing the necessary Azure infrastructure. This guide provides a detailed look at the components involved and how they work together to support your applications.

## Core Infrastructure Components

### Networking Architecture

Networking is a critical aspect of cloud infrastructure, ensuring secure and efficient communication between different parts of your application. Encore Cloud creates an isolated [Azure Virtual Network (VNet)][az-vnet] for each environment, which serves as a secure network boundary.

The network architecture is designed with reliability and security in mind. Each VNet spans across two Availability Zones within a single Azure region, providing redundancy and fault tolerance. If one zone experiences issues, your application can continue running in another zone, significantly reducing the risk of downtime. This multi-zone setup is crucial for maintaining high availability in production environments.

Within the VNet, Encore Cloud implements a three-tier architecture that carefully separates different components of your application into distinct subnet layers. This separation of concerns enhances both security and performance by controlling traffic flow between layers and limiting potential attack vectors. Each tier is configured with [Network Security Groups (NSGs)][az-nsg] to enforce these boundaries, creating a robust and secure networking foundation for your application.

```
┌─────────────────────────────────────────────────────────────────┐
│  Azure Virtual Network  (e.g. 10.0.0.0/16)                      │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Public Subnet  (e.g. 10.0.0.0/24)                       │   │
│  │  • Azure Application Gateway / Load Balancer (ingress)   │   │
│  │  • NAT Gateway (outbound for private subnets)            │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Compute Subnet  (e.g. 10.0.1.0/24)                      │   │
│  │  • AKS node pools / Container Apps environments          │   │
│  │  • Accepts inbound only from public subnet               │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Private Subnet  (e.g. 10.0.2.0/24)  [provisioned as    │   │
│  │  needed]                                                  │   │
│  │  • Azure Database for PostgreSQL (private endpoint)      │   │
│  │  • Azure Cache for Redis (private endpoint)              │   │
│  │  • No inbound internet access; compute subnet only       │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

#### Subnet Tiers

1. **Public Subnet**
   The public subnet contains the components that manage external traffic flow. At the forefront is the [Azure Application Gateway][az-appgw] (or Azure Load Balancer for simpler topologies), which serves as the entry point for all incoming traffic to your application. It intelligently distributes requests across your application instances, ensuring optimal performance and reliability.

   To enable outbound communication, the subnet includes a [NAT Gateway][az-natgw] that provides a secure pathway for resources in private subnets (like your compute instances) to access the internet while remaining protected from direct external access. This NAT Gateway acts as an intermediary, translating private IP addresses to public ones for outbound traffic while maintaining the security of your internal resources.

2. **Compute Subnet**
   The compute subnet is where your application's containers run, regardless of whether you're using AKS or Azure Container Apps as your container orchestration platform. This subnet is carefully isolated and configured to only accept incoming traffic from the Application Gateway in the public subnet. This strict traffic control ensures that your application containers can only be accessed through proper channels, protecting them from unauthorized direct access while still allowing legitimate requests to flow through seamlessly.

3. **Private Subnet** (provisioned as needed)
   The private subnet is a dedicated network segment designed to host your application's databases and caching systems. To maintain the highest level of security, this subnet operates in complete isolation from the internet, with no direct inbound or outbound internet connectivity. All managed services (PostgreSQL, Redis) are attached via [private endpoints][az-private-endpoint], ensuring traffic stays entirely within the VNet. Access to resources within the private subnet is strictly limited to traffic originating from the compute subnet, creating a secure enclave for your data layer.

### Container Management

Encore Cloud provisions an [Azure Container Registry (ACR)][az-acr] to store your application's Docker images. The registry is seamlessly integrated with your chosen compute platform and provides robust security features. Access to images is tightly controlled through Azure RBAC role assignments (specifically the `AcrPull` role), ensuring only authorized services and managed identities can pull or push container images. Additionally, ACR can be configured to perform automated vulnerability assessments on images as they are pushed to the registry, helping you maintain a secure application environment.

### Secrets Management

Managing sensitive information securely is crucial. Encore Cloud uses [Azure Key Vault][az-keyvault] to store and manage secrets, such as API keys and database credentials. Through deep integration with Azure Key Vault, Encore Cloud automatically retrieves secrets at runtime and injects them into your service's environment, making them easily accessible while maintaining strict security controls. All secrets are encrypted both at rest and in transit using Azure-managed or customer-managed keys, providing comprehensive protection for your sensitive data. The system implements fine-grained access controls via managed identity role assignments — each service is given precisely scoped permissions to access only the specific secrets it needs, ensuring that even if one service is compromised, the blast radius is contained and other secrets remain secure.

## Compute Options

Encore Cloud provisions one of two compute platforms for running your application containers, based on your choice:

### Azure Kubernetes Service (AKS)

When using AKS, Encore Cloud configures:

- **Cluster Setup**
  Encore Cloud provisions an AKS cluster with the [Azure CNI][az-aks-cni] networking plugin so that each pod receives an IP address directly from the VNet subnet, enabling fine-grained NSG control and seamless private endpoint connectivity. The cluster's internal DNS resolution is handled through CoreDNS, configured for optimal service discovery and name resolution within the cluster. Node pools are placed in the private compute subnet and are not directly reachable from the internet.

  Encore Cloud enables [Azure Workload Identity][az-workload-identity] on the cluster, which federates Kubernetes service accounts with Azure Managed Identity. This means pods can authenticate to Azure services (Key Vault, Service Bus, Blob Storage, etc.) using short-lived OIDC tokens rather than long-lived credentials stored as secrets.

- **Kubernetes Resources**
  Encore Cloud automatically manages all necessary Kubernetes resources for your application. Each service in your application is deployed as a separate Kubernetes Deployment, allowing for independent scaling and lifecycle management. These deployments are configured with appropriate resource requests, limits, and health checks to ensure reliable operation.

  Each service gets its own Kubernetes ServiceAccount annotated with the corresponding Azure Managed Identity client ID, providing secure, least-privilege access to Azure services. For sensitive data like API keys and credentials, Encore Cloud uses Kubernetes Secrets encrypted at rest, or fetches them directly from Azure Key Vault at runtime.

  To enable network connectivity, Encore Cloud creates Kubernetes Service resources for each of your application's services, providing stable networking endpoints for inter-service communication.

- **Load Balancer Integration**
  Encore Cloud manages complete load balancer integration for your AKS cluster. The [Application Gateway Ingress Controller (AGIC)][az-agic] is automatically installed and configured to handle ingress traffic. AGIC works in conjunction with the Azure Application Gateway to provide intelligent traffic routing, SSL/TLS termination, and Web Application Firewall (WAF) capabilities.

  The Application Gateway is automatically provisioned in the public subnet and configured with backend pools that target your service pods. Health probes are configured to maintain accurate health status for all targets. SSL/TLS certificates are managed through [Azure Key Vault integration][az-appgw-tls], ensuring all external traffic to your application is encrypted and certificates are automatically renewed.

- **Monitoring Setup**
  Encore Cloud automatically aggregates and sends metrics to your configured metrics destination. Azure Monitor is the native destination for custom metrics when running on Azure, providing real-time visibility into your application's performance.

  Container logs are forwarded to [Azure Monitor Logs (Log Analytics)][az-log-analytics] via the AKS diagnostic settings and the container insights add-on, enabling centralized log aggregation and analysis. Log streams are organized by service name and namespace, making it easy to search and analyze application behavior.

- **Service Accounts and Managed Identity**
  Encore Cloud implements a comprehensive service account management system that ensures secure and controlled access to Azure resources. Each service in your application receives its own dedicated Kubernetes service account, providing a unique identity for authentication and authorization.

  To enable secure interaction with Azure services, Encore Cloud maps each Kubernetes service account to a corresponding Azure User-Assigned Managed Identity using Workload Identity federation. This mapping allows pods to securely authenticate with Azure services without storing long-lived credentials.

  The managed identities are automatically configured with the minimum required permissions for each service's needs. This includes:
  - `Storage Blob Data Contributor` role on the service's Azure Blob Storage containers
  - `Azure Service Bus Data Owner` (or scoped Sender/Receiver) on the relevant Service Bus namespace
  - `Key Vault Secrets User` role on the Key Vault for secret retrieval
  - `Contributor` or scoped role on the PostgreSQL Flexible Server for database operations

  These role assignments are continuously updated as your application evolves, ensuring services always have the access they need while maintaining strong security boundaries.

### Azure Container Apps

When using Azure Container Apps, Encore Cloud configures:

- **Environment Setup**
  Encore Cloud provisions a [Container Apps Environment][az-aca] deployed into a dedicated subnet within the VNet, giving each container app a private IP address and full connectivity to private endpoints for databases, caches, and message brokers. The environment uses a workload profile that balances cost and performance for your workload.

- **Container App Deployments**
  Each Encore service is deployed as a separate Container App within the shared environment. Container Apps are configured with optimized scaling rules — scaling to zero in development environments to minimize cost, and maintaining a minimum replica count in production for availability. Each app is configured with appropriate health probes and resource allocations.

  Rolling deployments are used to ensure zero downtime during updates. New revisions are gradually introduced using traffic-splitting rules, allowing safe canary deployments and instant rollback if issues are detected.

- **IAM Configuration**
  Each Container App is assigned its own User-Assigned Managed Identity, providing a unique, auditable identity for every service. These identities are granted the minimum required permissions on Azure resources they interact with — Blob Storage, Service Bus, Key Vault, and databases — following the principle of least privilege.

- **Monitoring Setup**
  Container Apps emit logs and metrics to Azure Monitor Log Analytics automatically through the Container Apps environment's built-in diagnostics integration. Custom application metrics are exported to Azure Monitor using the `azure_monitor` metrics provider, enabling rich dashboards and alerting in the Azure portal.

All of these configurations are automatically maintained and updated by Encore Cloud as you develop your application, ensuring your infrastructure stays aligned with your application's needs.

## Managed Services

### Databases

Encore Cloud provisions [Azure Database for PostgreSQL Flexible Server][az-postgres] for databases, providing a robust and scalable database solution. Each database runs a recent PostgreSQL version to ensure compatibility with modern features while maintaining up-to-date security patches. The databases are provisioned with auto-scaling storage starting from a cost-effective compute tier (e.g., `Standard_D2s_v3`) that can scale up as your application's needs grow.

To protect your data, Encore Cloud configures automated daily backups with a 7-day retention period and supports point-in-time restore. Security is paramount — PostgreSQL Flexible Servers are integrated with the VNet via a [private endpoint][az-private-endpoint], meaning the server has no public internet endpoint whatsoever. Strict NSG rules ensure only the compute subnet can initiate connections to the database port (5432).

#### Database Access

Database access is managed through a comprehensive security model. At its core, Encore Cloud deploys [Emissary](https://github.com/encoredev/emissary), a secure socks proxy that enables safe database migrations while maintaining strict access controls. Each service in your application is assigned its own dedicated database role, providing granular control over data access and ensuring services can only interact with the data they need. Credentials are stored in Azure Key Vault and injected at runtime via the Encore secrets provider integration.

### Pub/Sub

Encore Cloud implements a robust messaging system using [Azure Service Bus][az-servicebus]. A dedicated Service Bus namespace is provisioned per environment. Within the namespace, Encore Cloud creates a **topic** for each Encore pub/sub topic declared in your application, and a **subscription** per subscriber service on that topic.

The Service Bus namespace is configured with the **Standard** tier (which supports topics and subscriptions) or **Premium** tier for production workloads that require private endpoints and message sizes greater than 256 KB. Dead-letter sub-queues are automatically enabled on each subscription to capture failed messages, enabling thorough analysis and debugging of messaging issues.

Each service in your application is granted precisely scoped role assignments (`Azure Service Bus Data Sender` for publishers, `Azure Service Bus Data Receiver` for subscribers) using managed identity, ensuring secure communication between components without the need to manage connection strings. Encore Cloud fully manages the creation and configuration of topics and subscriptions, streamlining setup and ongoing maintenance while maintaining optimal performance and reliability.

### Object Storage

Encore Cloud leverages [Azure Blob Storage][az-blob] for object storage, providing a comprehensive solution for your application's storage needs. When you declare storage buckets in your application, Encore Cloud automatically provisions dedicated **Azure Storage Accounts** with a **Blob Service** container per Encore bucket, using globally unique names to ensure uniqueness across Azure.

Each service in your application is granted precisely scoped role assignments (`Storage Blob Data Contributor` or `Storage Blob Data Reader`) on the relevant containers, following the principle of least privilege. For public buckets, Encore Cloud can optionally integrate with [Azure CDN][az-cdn] to create a global content delivery network, significantly improving access speeds for your users worldwide. Each container is accessible through a predictable URL pattern (`https://<account>.blob.core.windows.net/<container>`), making it simple to manage and access stored content.

### Caching

Encore Cloud uses [Azure Cache for Redis][az-redis] to provide a high-performance caching solution. Each cache starts with a cost-effective SKU (e.g., `Standard C1`) that can be upgraded as your application's caching needs grow. To ensure maximum reliability, caches are configured in zone-redundant mode across availability zones where supported, providing both high availability and fault tolerance. In the event of failures, automatic failover ensures your application experiences no disruption in service.

Security is maintained through Redis Authentication and TLS in-transit encryption. The Redis cache is connected to the VNet via a private endpoint, ensuring cache traffic never traverses the public internet. Access credentials are stored in Azure Key Vault and automatically managed by Encore Cloud.

### Cron Jobs

Encore Cloud provides a streamlined approach to scheduled tasks that prioritizes security and simplicity. Each cron job is executed through authenticated API requests that are cryptographically signed to verify their authenticity. The system performs rigorous source verification to ensure all scheduled tasks originate exclusively from Encore Cloud's cron functionality, preventing unauthorized execution attempts. This implementation requires no additional infrastructure components, making it both cost-effective and easy to maintain while ensuring your scheduled tasks run reliably and securely.

## Identity & Access Model

Encore Cloud uses [Azure Managed Identity][az-managed-identity] as the cornerstone of its security model, eliminating the need for long-lived credentials in your workloads:

- **User-Assigned Managed Identities** are provisioned per service, giving each a stable, auditable identity independent of the compute lifecycle.
- **Workload Identity** (AKS) or **built-in managed identity** (Container Apps) federates the Kubernetes/container identity to Azure AD, allowing pods to obtain short-lived Azure AD tokens via the OIDC token projection.
- **Role assignments** are scoped as narrowly as possible — to individual storage containers, Service Bus topics/subscriptions, Key Vault secrets, and database instances — rather than granted at the subscription or resource group level.
- **DefaultAzureCredential** in the Encore runtime automatically resolves the correct credential chain: managed identity in production, Azure CLI or environment credentials in local development.

## Cost & Permissions Notes

**Minimum Azure permissions for Encore Cloud deployment:**

To allow Encore Cloud to provision and manage infrastructure on your behalf, the deployment principal (service principal or managed identity used by Encore Cloud's control plane) requires the following:

| Scope | Role / Permission |
|---|---|
| Subscription or Resource Group | `Contributor` (to create/modify resources) |
| Subscription or Resource Group | `User Access Administrator` (to create role assignments for managed identities) |
| Azure AD | `Application Administrator` or the ability to create service principals (for workload identity federation) |

For a production hardened setup, you can scope `Contributor` to a dedicated resource group per environment, combined with a custom role that permits only the resource types Encore Cloud manages (`Microsoft.Network/*`, `Microsoft.ContainerService/*`, `Microsoft.DBforPostgreSQL/*`, `Microsoft.Cache/*`, `Microsoft.ServiceBus/*`, `Microsoft.Storage/*`, `Microsoft.KeyVault/*`, `Microsoft.ContainerRegistry/*`, `Microsoft.ManagedIdentity/*`).

**Estimated cost drivers** (varies by region and SKU):
- AKS cluster management fee + node VM costs (waived for free tier clusters in some regions)
- Azure Database for PostgreSQL Flexible Server compute + storage
- Azure Cache for Redis Standard tier
- Azure Service Bus Standard/Premium namespace
- Azure Container Registry Basic/Standard tier
- Application Gateway (WAF_v2 SKU for production)
- NAT Gateway hourly + data processed charges

[az-vnet]: https://learn.microsoft.com/en-us/azure/virtual-network/virtual-networks-overview
[az-nsg]: https://learn.microsoft.com/en-us/azure/virtual-network/network-security-groups-overview
[az-acr]: https://learn.microsoft.com/en-us/azure/container-registry/container-registry-intro
[az-aks]: https://learn.microsoft.com/en-us/azure/aks/intro-kubernetes
[az-aks-cni]: https://learn.microsoft.com/en-us/azure/aks/configure-azure-cni
[az-workload-identity]: https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview
[az-aca]: https://learn.microsoft.com/en-us/azure/container-apps/overview
[az-appgw]: https://learn.microsoft.com/en-us/azure/application-gateway/overview
[az-agic]: https://learn.microsoft.com/en-us/azure/application-gateway/ingress-controller-overview
[az-appgw-tls]: https://learn.microsoft.com/en-us/azure/application-gateway/key-vault-certs
[az-natgw]: https://learn.microsoft.com/en-us/azure/nat-gateway/nat-overview
[az-private-endpoint]: https://learn.microsoft.com/en-us/azure/private-link/private-endpoint-overview
[az-keyvault]: https://learn.microsoft.com/en-us/azure/key-vault/general/overview
[az-postgres]: https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/overview
[az-servicebus]: https://learn.microsoft.com/en-us/azure/service-bus-messaging/service-bus-messaging-overview
[az-blob]: https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction
[az-cdn]: https://learn.microsoft.com/en-us/azure/cdn/cdn-overview
[az-redis]: https://learn.microsoft.com/en-us/azure/azure-cache-for-redis/cache-overview
[az-managed-identity]: https://learn.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview
[az-log-analytics]: https://learn.microsoft.com/en-us/azure/azure-monitor/logs/log-analytics-overview
