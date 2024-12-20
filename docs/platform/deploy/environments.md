---
seotitle: Environments – Creating local, preview, and prod environments
seodesc: Learn how to create all the environments you need for your backend application, local, preview, testing and production. Here's how you keep them in sync!
title: Creating & configuring environments
subtitle: Get the environments you need, without the work
lang: platform
---

Encore automatically sets up and manages different environments for your application (local, preview, testing, and production). Each environment is:
- Fully isolated
- Automatically provisioned
- Always in sync with your codebase
- Configured with appropriate infrastructure for its purpose

## Environment Types

Encore has four types of environments:
- `production`
- `development`
- `preview`
- `local`

Some environment types differ in how infrastructure is provisioned:
- `local` is provisioned by Encore's Open Source CLI using local versions of infrastructure.
- `preview` environments are provisioned in Encore Cloud hosting and are optimized to be cost-efficient and fast to provision.
- `production` and `development` environments are provisioned by Encore Cloud, either in your [cloud account](/docs/platform/deploy/own-cloud) or using Encore Cloud's free development hosting. Both environment types offer the same infrastructure options when deployed using your own cloud account.
  
Environment type is also used for [Secrets management](/docs/ts/primitives/secrets), allowing you to configure different secrets for different environment types. Therefore, you can easily configure different secrets for your `production` and `development` environments.

## Creating environments

1. Open your app in the [Encore Cloud dashboard](https://app.encore.cloud)
2. Go to **Environments** > **Create env**
3. Configure your environment:
   - Name your environment
   - Choose type: **Production** or **Development** (see [Environment Types](#environment-types))
   - Set deploy trigger: Git branch or manual
   - Configure infrastructure approval: automatic or manual
   - Select cloud provider
   - Choose process allocation: single or separate processes

![Creating an environment](/assets/docs/createenv.png "Creating an environment")

### Configuring deploy trigger

When using GitHub, you can configure Encore Cloud to automatically trigger deploys when you push to a specific branch name.

To configure which branch name is used to trigger deploys, open your app in the [Encore Cloud dashboard](https://app.encore.cloud) and go to the **Overview** page for your intended environment. Click on **Settings** and then in the section **Branch Push** configure the `Branch name`  and hit save.

### Configuring infrastructure approval

For some environments you may want to enforce infrastructure approval before deploying. You can configure this in the **Settings** > **Infrastructure Approval** section for your environment.

When infrastructure approval is enabled, an application **Admin** will need to manually approve the infrastructure changes before the deployment can proceed.

### Configuring process allocation

Encore Cloud offers flexible process allocation options:
- **Single process**: All services run in one process (simpler, lower cost)
- **Separate processes**: Each service runs independently (better isolation, independent scaling)

Choose your preferred deployment model when creating each environment. You can use different models for production and development environments without changing any code.

<img src="/assets/docs/microservices-process-allocation.png" title="Microservices - Process Allocation" />

## Setting a Primary environment

Every Encore app has a configurable Primary environment that serves as the default for:
- App insights in the Encore Cloud dashboard
- API documentation
- CLI functionality (like API client generation)

**Configuring your Primary environment:**
1. Open your app in the [Encore Cloud dashboard](https://app.encore.cloud)
2. Navigate to **Settings** > **General** > **Primary Environment**
3. Select your desired environment from the dropdown
4. Click **Update**
