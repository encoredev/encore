---
seotitle: Cloudflare R2 Infrastructure on Encore Cloud
seodesc: A comprehensive guide to how Encore Cloud provisions and manages Cloudflare R2 infrastructure for your applications
title: Cloudflare R2 Buckets
lang: platform
---

Encore Cloud simplifies the process of using Cloudflare R2 for object storage by automatically provisioning and managing the necessary infrastructure. This guide provides setup instructions and details on how Encore Cloud manages your Cloudflare R2 infrastructure.

## Setup Process

### 1. Cloudflare Account Connection

To connect your Cloudflare account to Encore Cloud:

1. Create a Cloudflare API token using the **Create Additional Tokens** button in the Cloudflare dashboard 

2. Add the the following permissions:
   - Zone > Zone: Read
   - Zone > DNS: Edit
   - Account > Workers R2 Storage: Edit

3. Add the token in the Encore Cloud dashboard:
   - Navigate to App Settings > Integrations > Cloudflare
   - Click "Connect Account"
   - Provide an account name and your API token

### 2. Environment Configuration

When creating a new environment:

1. Select your preferred cloud provider
2. Choose "Cloudflare R2" as the object storage provider
3. Configure the following R2-specific settings:
   - Token: Your Cloudflare API token
   - Account: Your Cloudflare account
   - Zone: The domain zone for public bucket URLs
   - Region: Your preferred R2 storage region

## Managed Features

### Bucket Management

Encore Cloud provides comprehensive bucket management capabilities that adapt to your application's needs. When you define storage requirements in your application, Encore Cloud automatically provisions the necessary R2 buckets with appropriate configurations. Each bucket is created with carefully configured policies and access controls to ensure secure yet efficient access to your stored objects. 

### Public Access Configuration

When working with public buckets, Encore Cloud handles all aspects of public access configuration automatically. Each bucket is assigned a unique subdomain that is automatically provisioned and configured in your DNS settings. The bucket is seamlessly integrated with Cloudflare's global CDN network, ensuring fast content delivery worldwide. Encore Cloud also configures optimal caching rules to maximize performance while maintaining appropriate cache invalidation policies. This comprehensive setup ensures your public content is served efficiently and securely through Cloudflare's infrastructure.

### Security Controls

Encore Cloud implements a comprehensive multi-layered security model to protect your R2 storage. At the bucket level, fine-grained access controls ensure that only authorized services can perform specific operations on each bucket. Each service in your application receives its own unique set of credentials, preventing any unauthorized cross-service access. These credentials are securely distributed to the appropriate services through Encore Cloud's built-in secrets management system, which handles the entire credential lifecycle.

All these configurations are automatically maintained and updated by Encore Cloud as you develop your application, ensuring your infrastructure stays aligned with your application's needs.

[cloudflare-r2]: https://developers.cloudflare.com/r2/
[cloudflare-cdn]: https://developers.cloudflare.com/cdn/