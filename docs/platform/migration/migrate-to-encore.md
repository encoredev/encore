---
title: Migrating an existing system to Encore
subtitle: Approaches for adopting Encore
seotitle: How to migrate your existing system to Encore
seodesc: Learn how to migrate your application to Encore incrementally, and unlock Encore's powerful set of development tools for your team.
lang: platform
---

By building your application with the Encore open-source framework, you unlock powerful features such as the [local development tools](/docs/ts/observability/dev-dash), [automatic infrastructure provisioning](/docs/platform/infrastructure/infra), [distributed tracing](/docs/ts/observability/tracing), and [service catalog](/docs/ts/observability/service-catalog).

**The good news: you don't need a complete rewrite.** This guide shows you how to adopt Encore incrementally, so you can start benefiting immediately while gradually migrating your existing system.

## Why incremental migration?

Incremental migration is more reliable than a complete rewrite. Here's why:

- **Immediate value** - Start benefiting from Encore's features when developing your next new service.
- **Lower risk** - Small, controlled changes instead of a single high-stakes big-bang launch.
- **Ship faster** - Deliver improvements incrementally rather than waiting for a complete rewrite.

## Choose your migration strategy

We recommend two approaches:

1. **Service by service** (Recommended) - Migrate services one at a time. Run Encore alongside your legacy system, integrated via APIs.
2. **Forklift migration** - Move your entire application in one shot using a catch-all handler, then refactor incrementally.

<img src="/assets/docs/migration-diagram.png" title="Migration options" className="noshadow"/>

### Need help?

We've helped 100+ teams adopt Encore and we're happy to answer your questions and provide advice to help you with your migration.

[Email us](mailto:hello@encore.dev) to ask questions, or [book a 1:1 call](https://encore.dev/book) to discuss your specific situation.

**Enterprise customers**: Encore Cloud can adapt to your unique infrastructure—Kubernetes clusters, VPCs, security policies, and compliance needs—typically within days. [Contact us](https://encore.dev/book) to discuss your requirements.

## Service by service migration (Recommended)

Migrate services one at a time while your Encore application runs alongside your legacy system, integrated through APIs.

### Key benefits

- **Full Encore features immediately** - Get automatic infrastructure provisioning, distributed tracing, and architecture diagrams for each migrated service.
- **Independent services** - Each service is self-contained with no cross-application dependencies.
- **Simple integration** - Services communicate via APIs.
- **Flexible deployment** - Deploy to your existing Kubernetes cluster, or let Encore Cloud set up a new project in your cloud (AWS/GCP).
- **Better developer experience** - Start building with modern tooling right away.

### Deployment options

Choose how to deploy your Encore application:

- **Your Kubernetes cluster** - Deploy directly to your existing Kubernetes infrastructure. Run Encore alongside legacy systems securely in the same environment.
- **Encore-managed in your cloud account** - Let Encore handle all infrastructure provisioning and management in your AWS or GCP account, and deploy within your existing VPC and security setup.

**Enterprise**: We can adapt to your specific network topology, security policies, and compliance requirements — typically within days. [Contact us](https://encore.dev/book) to discuss your requirements.

_Google Cloud example architecture:_

<img src="/assets/docs/gcp-diagram.png" title="Migration options" className="noshadow"/>

### Which services to migrate first?

Start small and build confidence:

- **Low-risk, high-value** - Validate the approach before tackling complex systems.
- **Frequently changed** - Get immediate developer experience benefits where it matters most.
- **Clear boundaries** - Services with well-defined APIs are easier to migrate.
- **Fewer dependencies** - Less connected to legacy infrastructure means simpler migration.

### Practical steps

#### 1. Create an Encore app and integrate with GitHub

The first step in any project is to create an Encore app. If you've not tried Encore before, we recommend starting by following the [Quick Start Guide](/docs/ts/quick-start).

Once you've created you app, [integrate it with your GitHub repository](/docs/platform/integrations/github) and you'll get automatic [Preview Environments](/docs/platform/deploy/preview-environments) for every Pull Request.

#### 2. Build your services and APIs

Since Encore is designed to build distributed systems, it should be straightforward to build a new system that integrates with your existing backend through APIs. See the [defining APIs documentation](/docs/ts/primitives/defining-apis) for more details.

Should you want to accept webhooks, that's simple to do using Encore's [Raw endpoints](/docs/ts/primitives/raw-endpoints).

You can also generate API clients in several languages, which makes it simple to integrate with frontends or other systems. See the [Client Generation documentation](/docs/ts/cli/client-generation) for more details.

#### 3. Deploy alongside your existing backend

**Deploy to Kubernetes**

Encore Cloud can deploy directly to your existing Kubernetes cluster:

- Run in the same secure environment as your legacy systems
- Services communicate within your existing VPC
- Gradually shift traffic using your load balancer or service mesh
- Use your current cost management and billing
- Maintain compliance with your governance policies

[Contact us](https://encore.dev/book) to discuss Kubernetes deployment.

**Deploy to new Encore-managed infrastructure in your cloud**

[Connect your AWS or GCP account](/docs/platform/deploy/own-cloud) to deploy in your existing environment:

- Same VPC as your legacy backend (or a new one)
- Your current cost management and billing
- Maintain compliance with your governance policies

See [infrastructure docs](/docs/platform/infrastructure/infra#production-infrastructure) for details.

#### Integration patterns

Your Encore and legacy systems communicate through APIs:

- **Legacy → Encore**: Use Encore-generated API clients
- **Encore → Legacy**: Use your existing API communication protocol (Encore is not opinionated)
- **Authentication**: Choose to deploy an authentication gateway in front of Encore or implement authentication directly in your Encore app
- **Events**: Use Encore's built-in Pub/Sub support for loose coupling

#### 4. Expand your migration

Continue migrating services incrementally. Strategies to consider:

- **Related services**: Migrate services that interact frequently to maximize tracing benefits
- **High-churn areas**: Move frequently changed services first
- **New features**: Build new functionality in Encore from the start
- **Critical paths**: Once confident, migrate business-critical services

## Forklift migration using a catch-all handler

Should you prefer, you can use a forklift migration strategy to move your entire application to Encore in one step by wrapping your existing HTTP router in a catch-all handler.

### When to consider this approach

This strategy works well when:

- Your existing system is a monolith or smaller distributed system
- The codebase relies primarily on infrastructure primitives supported by Encore (microservices, databases, pub/sub, caching, object storage, cron jobs, and secrets)
- You want to quickly consolidate everything in one place
- You're prepared to incrementally refactor to unlock full Encore features like tracing and automatic API documentation

### Trade-offs

**Benefits:**

- **Quick consolidation**: Get everything in one place from the start.
- **Immediate access to core features**: Quickly use Encore's CI/CD system, secrets manager, and deployment capabilities.
- **Single codebase**: Simplified development and deployment workflow.

**Limitations:**

- **Limited initial visibility**: Advanced features like [distributed tracing](/docs/ts/observability/tracing) and [architecture diagrams](/docs/ts/observability/encore-flow) require the [Encore application model](/docs/ts/concepts/application-model) and won't work immediately.
- **Requires refactoring**: You'll need to incrementally break out endpoints to unlock full Encore capabilities.
- **All-at-once risk**: Unlike service-by-service migration, this is a bigger initial change.

### Practical steps

Here follows a quick summary of the high-level steps of a forklift migration. Find more in-depth instructions in the full [forklift migration guide](/docs/ts/migration/express-migration#forklift-migration-quick-start).

#### 1. Create an app and structure your code

To start, create an Encore application and copy over the code from your existing repository. In order to run your application with Encore, it needs to follow the expected [application structure](/docs/ts/primitives/app-structure), which involves placing the `encore.app` and `package.json` files in the repository root. This should be straightforward to do with minor modifications.

As an example, a single service application might look like this on disk:

```
/my-app
├── package.json
├── encore.app
├──  // ... other project files
│
├── encore.service.ts    // defines your service root
├── api.ts               // API endpoints
├── db.ts                // Database definition
```

You can also have services nested inside a `backend` folder if you prefer.

#### 2. Create a catch-all handler for your HTTP router

Now let's mount your existing HTTP router under a [Raw endpoint](/docs/ts/primitives/raw-endpoints), which is an Encore API endpoint type that gives you access to the underlying HTTP request.

Here's a basic code example:

```ts
import { api } from "encore.dev/api";

export const migrationHandler = api.raw(
  { expose: true, method: "*", path: "/api/*path" },
  async (req, resp) => {
    // pass request to existing router
  }
);
```

By mounting your existing HTTP router in this way, it will work as a catch-all handler for all HTTP requests and responses. This should make your application deployable through Encore with little refactoring.

#### 3. Iteratively fix remaining compilation errors

Exactly what remains to make your application deployable with Encore will depend on your specific app.
As you run your app locally, using `encore run`, Encore will parse and compile it, and give you compilation errors to inform what needs to be adjusted.

By iteratively making adjustments, you should relatively quickly be able to get your application up and running with Encore.

#### 4. Refactor incrementally to unlock Encore features

Once your application is deployed, gradually break out specific endpoints using Encore's [API declarations](/docs/ts/primitives/defining-apis) and introduce infrastructure declarations using the Encore backend frameworks. This incremental refactoring will:

- Enable Encore to understand your application structure
- Unlock powerful features like distributed tracing and architecture diagrams
- Improve observability and debugging capabilities
- Make your codebase more maintainable and easier to evolve

Start with the most frequently modified endpoints or the most critical user flows to maximize the value of refactoring efforts.

## Conclusion

Incremental migration lets you adopt Encore without the risk of a complete rewrite.

**Service by service migration** is the recommended approach—it gives you Encore's full feature set immediately while running safely alongside your existing systems.

**Enterprise customers** benefit from flexible deployment options, including Kubernetes integration and customization that typically takes just days to set up.

### Have questions?

We've helped 100+ teams adopt Encore and we're happy to answer your questions and provide advice to help you with your migration.

- [Book a call](/book) to get 1:1 assistance
- [Email us](mail:hello@encore.dev) to ask questions
- [Join Discord](https://encore.dev/discord) to discuss with other developers using Encore
