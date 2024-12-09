---
seotitle: AWS Infrastructure on Encore Cloud
seodesc: A comprehensive guide to how Encore Cloud provisions and manages AWS infrastructure for your applications
title: AWS Infrastructure
subtitle: Understanding your application's AWS infrastructure
lang: platform
---
Encore Cloud simplifies the process of deploying applications by automatically provisioning and managing the necessary AWS infrastructure. This guide provides a detailed look at the components involved and how they work together to support your applications.

## Core Infrastructure Components

### Networking Architecture

Networking is a critical aspect of cloud infrastructure, ensuring secure and efficient communication between different parts of your application. Encore Cloud creates an isolated [Virtual Private Cloud (VPC)][aws-vpc] for each environment, which serves as a secure network boundary.

The network architecture is designed with reliability and security in mind. Each VPC spans across two Availability Zones (AZs), providing redundancy and fault tolerance. If one AZ experiences issues, your application can continue running in the other AZ, significantly reducing the risk of downtime. This multi-AZ setup is crucial for maintaining high availability in production environments.

Within the VPC, Encore Cloud implements a three-tier architecture that carefully separates different components of your application into distinct subnet layers. This separation of concerns enhances both security and performance by controlling traffic flow between layers and limiting potential attack vectors. Each tier is configured with specific security groups and network ACLs to enforce these boundaries, creating a robust and secure networking foundation for your application.

#### Subnet Tiers

1. **Public Subnet**
   The public subnet contains several key components that manage external traffic flow. At the forefront is the Application Load Balancer (ALB), which serves as the entry point for all incoming traffic to your application. The ALB intelligently distributes requests across your application instances, ensuring optimal performance and reliability.

   To enable outbound communication, the subnet includes an Internet Gateway that allows your application components to securely connect to external services and APIs. Working alongside it is a NAT Gateway, which provides a secure pathway for resources in private subnets (like your compute instances) to access the internet while remaining protected from direct external access. This NAT Gateway acts as an intermediary, translating private IP addresses to public ones for outbound traffic while maintaining the security of your internal resources.

2. **Compute Subnet**
   The compute subnet is where your application's containers run, regardless of whether you're using Fargate or EKS as your container orchestration platform. This subnet is carefully isolated and configured to only accept incoming traffic from the Application Load Balancer in the public subnet. This strict traffic control ensures that your application containers can only be accessed through proper channels, protecting them from unauthorized direct access while still allowing legitimate requests to flow through seamlessly.

3. **Storage Subnet** (provisioned as needed)
   The storage subnet is a dedicated network segment designed to host your application's databases and caching systems. To maintain the highest level of security, this subnet operates in complete isolation from the internet, with no direct inbound or outbound connectivity. Access to resources within the storage subnet is strictly limited to traffic originating from the compute subnet, creating a secure enclave for your data layer. This architecture ensures that your sensitive data remains protected while still being readily accessible to your application's services running in the compute tier.

### Container Management

Encore Cloud provisions an [Elastic Container Registry (ECR)][aws-ecr] to store your application's Docker images. The registry is seamlessly integrated with your chosen compute platform and provides robust security features. Access to images is tightly controlled through comprehensive access controls, ensuring only authorized users and services can pull or push container images. Additionally, ECR automatically scans all images for known security vulnerabilities as they are pushed to the registry, helping you maintain a secure application environment by identifying potential risks before deployment.

### Secrets Management

Managing sensitive information securely is crucial. Encore Cloud uses [AWS Secrets Manager][aws-secrets] to store and manage secrets, such as API keys and database credentials. Through deep integration with AWS Secrets Manager, Encore Cloud automatically injects secrets directly into your service's environment variables at runtime, making them easily accessible while maintaining strict security controls. All secrets are encrypted both at rest and in transit using industry-standard encryption algorithms, providing comprehensive protection for your sensitive data. The system implements fine-grained access controls, where each service is given precisely scoped permissions to access only the specific secrets it needs. This ensures that even if one service is compromised, the blast radius is contained and other secrets remain secure.

## Compute Options

Encore Cloud provisions one of two compute platforms for running your application containers, based on your choice:

### AWS Fargate

When using Fargate, Encore Cloud configures:

- **Task Definitions**
  Task definitions are meticulously configured to ensure optimal performance and reliability of your services. Each service's container settings are fine-tuned based on its specific requirements, including memory allocation, CPU utilization, and networking parameters. Comprehensive health check configurations monitor the service's status, enabling quick detection and recovery from any issues. Environment variables are securely injected from AWS Secrets Manager at runtime, providing your services with the credentials and configuration they need while maintaining security. The task definitions are also integrated with AWS Service Discovery, enabling automatic service registration and allowing for seamless service-to-service communication within your application.

- **Fargate Services**
  Fargate services are configured with sophisticated deployment strategies that ensure zero downtime during updates. When deploying new versions of your services, Encore Cloud orchestrates a rolling update process where new tasks are gradually introduced while old ones are removed, maintaining consistent availability throughout the deployment.

  Each service is automatically integrated with Application Load Balancer target groups, enabling intelligent request routing and load distribution. The load balancer continuously monitors the health of your service instances and automatically routes traffic only to healthy targets.

  To ensure smooth service startup, appropriate health check grace periods are configured. This gives your services adequate time to initialize and warm up before receiving traffic, preventing premature health check failures during deployment or scaling events.

- **IAM Configuration**
  Encore Cloud implements a comprehensive IAM security model by creating unique execution roles for each task definition. These roles are automatically configured with precisely scoped permissions that enable secure access to required AWS services. The execution roles allow containers to pull images from ECR and write operational logs to CloudWatch for monitoring and debugging. They also grant access to assigned AWS resources like S3 buckets and SQS queues that the service needs to interact with. Additionally, the roles are configured to securely retrieve secrets from AWS Secrets Manager at runtime, enabling safe storage and access of sensitive configuration data. This granular permission model follows security best practices by providing each service with the minimum privileges required for operation.

- **Network Integration**
Fargate tasks are strategically placed within private compute subnets, ensuring they remain isolated from direct internet access while maintaining the ability to communicate with other application components. The associated security groups are configured with precise rules that govern network traffic. These rules allow inbound traffic exclusively from the Application Load Balancer, ensuring that your services can only be accessed through the properly configured entry point. For outbound connectivity, the security groups permit traffic to flow to your databases and caching layers, enabling your services to interact with these essential backend resources while maintaining a secure network boundary.

### Amazon EKS

When using EKS, Encore Cloud configures:

- **Cluster Setup**
  Encore Cloud configures the core networking components required for cluster operation. The VPC CNI (Container Network Interface) is configured to enable pod networking within the cluster, allowing pods to communicate efficiently using the underlying AWS VPC networking capabilities. This includes setting up IP address management and network policy enforcement.

  The cluster's internal DNS resolution is handled through CoreDNS, which is configured for optimal service discovery and name resolution within the cluster. CoreDNS settings are tuned to provide fast and reliable DNS lookups while maintaining reasonable cache sizes and query limits.


- **Kubernetes Resources**
  Encore Cloud automatically manages all necessary Kubernetes resources for your application. Each service in your application is deployed as a separate Kubernetes Deployment, allowing for independent scaling and lifecycle management. These deployments are configured with appropriate resource requests, limits, and health checks to ensure reliable operation.

  For authentication and authorization, Encore Cloud implements IAM Roles for Service Accounts (IRSA), providing secure access to AWS services. Each service gets its own service account with precisely scoped IAM roles, following the principle of least privilege.

  For sensitive data like API keys and credentials, Encore Cloud uses Kubernetes Secrets, which are encrypted at rest and only accessible to authorized services.

  To enable network connectivity, Encore Cloud creates Kubernetes Service resources for each of your application's services, providing stable networking endpoints for inter-service communication. 

- **Load Balancer Integration**
  Encore Cloud manages the complete load balancer integration for your EKS cluster. The AWS Load Balancer Controller is automatically installed and configured to handle ingress traffic for your application. This controller works in conjunction with the Application Load Balancer (ALB) to provide intelligent traffic routing and SSL/TLS termination.

  The ALB Ingress Controller is configured to automatically create and manage Application Load Balancers based on your application's needs. It handles the creation and configuration of target groups, ensuring traffic is properly distributed across your service pods. The controller also manages the lifecycle of these resources, automatically cleaning up unused resources to prevent waste.

  Target group binding is automatically configured to map your Kubernetes services to the appropriate ALB target groups. This ensures that traffic is correctly routed to the right pods and that health checks are properly configured to maintain high availability.

  For secure communication, Encore Cloud automatically manages SSL/TLS certificates through AWS Certificate Manager. These certificates are automatically provisioned, renewed, and attached to your load balancers, ensuring all external traffic to your application is encrypted. The system also handles certificate rotation and updates transparently, maintaining secure communication without manual intervention.

- **Monitoring Setup**
  Encore Cloud automatically aggregates and sends metrics to your configured metrics destination, providing you with real-time visibility into your application's performance.

  In addition to metrics, Encore Cloud configures the CloudWatch Logs agent to capture and forward all container logs. The logs are structured and organized by service name, making it easy to search and analyze application behavior. Log streams are automatically created for each container, and log retention policies are configured to help manage storage costs while maintaining necessary historical data.

- **Service Accounts**
  Encore Cloud implements a comprehensive service account management system that ensures secure and controlled access to resources. Each service in your application receives its own dedicated Kubernetes service account, providing a unique identity for authentication and authorization purposes.

  To enable secure interaction with AWS services, Encore Cloud maps each Kubernetes service account to a corresponding IAM role using IAM Roles for Service Accounts (IRSA). This mapping allows pods to securely authenticate with AWS services without storing long-lived credentials.

  The IAM roles are automatically configured with the minimum required permissions for each service's needs. This includes access to service-specific S3 buckets for object storage operations, permissions to publish and subscribe to SQS queues and SNS topics, ability to retrieve secrets from AWS Secrets Manager, and secure access to assigned database instances. These permissions are continuously updated as your application evolves, ensuring services always have the access they need while maintaining strong security boundaries.

All of these configurations are automatically maintained and updated by Encore Cloud as you develop your application, ensuring your infrastructure stays aligned with your application's needs.

## Managed Services

### Databases
Encore Cloud provisions [Amazon RDS][aws-rds] for PostgreSQL databases, providing a robust and scalable database solution. Each database runs the latest PostgreSQL version to ensure compatibility with modern features while maintaining up-to-date security patches. The databases are provisioned on auto-scaling capable instances, starting with db.m5.large configurations that can seamlessly scale up as your application's needs grow.

To protect your data, Encore Cloud configures automated daily backups with a 7-day retention period. Security is paramount, so databases are strategically placed within private subnets and protected by comprehensive access controls. This network isolation combined with strict security rules ensures your data remains secure while still being accessible to your application's services.

#### Database Access
Database access is managed through a comprehensive security model. At its core, Encore Cloud deploys [Emissary](https://github.com/encoredev/emissary), a secure socks proxy that enables safe database migrations while maintaining strict access controls. Each service in your application is assigned its own dedicated database role, providing granular control over data access and ensuring services can only interact with the data they need. For enhanced security, all databases are placed in private subnets, completely isolated from direct internet access. This multi-layered approach creates a secure foundation for your application's data access needs while maintaining operational flexibility.

### Pub/Sub
Encore Cloud implements a robust messaging system using [SQS][aws-sqs] and [SNS][aws-sns]. The system automatically configures dead-letter queues to capture failed messages, enabling thorough analysis and debugging of messaging issues. Each service in your application receives precisely scoped IAM permissions to publish and consume messages, ensuring secure communication between components. Encore Cloud fully manages the creation and configuration of subscriptions and topics, streamlining the setup and ongoing maintenance of your messaging infrastructure while maintaining optimal performance and reliability.

### Object Storage
Encore Cloud leverages [S3][aws-s3] for object storage, providing a comprehensive solution for your application's storage needs. When you declare storage requirements in your application, Encore Cloud automatically provisions dedicated S3 buckets with unique names to ensure global uniqueness across AWS. Each service in your application receives precisely scoped permissions to perform storage operations, following the principle of least privilege. For public buckets, Encore Cloud automatically integrates with CloudFront to create a global content delivery network, significantly improving access speeds for your users worldwide. Each bucket is assigned its own unique domain name, making it simple to manage and access stored content while maintaining a clear organizational structure.

### Caching
Encore Cloud uses [ElastiCache for Redis][aws-redis] to provide a high-performance caching solution. The service starts with cache.m6g.large instances that can automatically scale up as your application's needs grow. To ensure maximum reliability, caches are configured with Multi-AZ replication across availability zones, providing both high availability and fault tolerance. In the event of any failures, automatic failover capabilities ensure your application experiences no disruption in service.

Security is maintained through Redis Access Control Lists (ACLs), which provide fine-grained control over who can access your cache and what operations they can perform. The entire system is configured for high availability, with monitoring and alerting in place to maintain optimal performance and uptime. This comprehensive setup ensures your application's caching layer remains fast, secure, and always available.

### Cron Jobs
Encore Cloud provides a streamlined approach to scheduled tasks that prioritizes security and simplicity. Each cron job is executed through authenticated API requests that are cryptographically signed to verify their authenticity. The system performs rigorous source verification to ensure all scheduled tasks originate exclusively from Encore Cloud's cron functionality, preventing unauthorized execution attempts. This elegant implementation requires no additional infrastructure components, making it both cost-effective and easy to maintain while ensuring your scheduled tasks run reliably and securely.

[aws-vpc]: https://docs.aws.amazon.com/vpc/latest/userguide/what-is-amazon-vpc.html
[aws-fargate]: https://aws.amazon.com/fargate/
[aws-eks]: https://aws.amazon.com/eks/
[aws-secrets]: https://aws.amazon.com/secrets-manager/
[aws-rds]: https://aws.amazon.com/rds/postgresql/
[aws-sqs]: https://aws.amazon.com/sqs/
[aws-sns]: https://aws.amazon.com/sns/
[aws-s3]: https://aws.amazon.com/s3/
[aws-redis]: https://aws.amazon.com/elasticache/redis/
[aws-ecr]: https://aws.amazon.com/ecr/
[aws-alb]: https://aws.amazon.com/elasticloadbalancing/application-load-balancer/
