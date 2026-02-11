---
seotitle: Benefits of using Encore.ts
seodesc: Get to know the benefits of using Encore's Backend Framework for TypeScript to build cloud-native backend applications.
title: Benefits of using Encore.ts
lang: ts
---

## Integrated developer experience for enhanced productivity

- **Local development with instant infrastructure**: Encore automatically sets up necessary infrastructure as you develop.
- **Rapid feedback**: Catch issues early with type-safe infrastructure, avoiding slow deployment cycles.
- **No manual configuration required**: No need for Infrastructure-as-Code. Your code is the single source of truth.
- **Unified codebase**: One codebase for all environments; local, preview, and cloud.
- **Cloud-agnostic by default**: Encore.ts provides an abstraction layer on top of the cloud provider's APIs, so you avoid becoming locked in to a single cloud.
- **Evolve infrastructure without code changes**: As requirements evolve, you can change the provisioned infrastructure without needing application code changes. Either using the Open Source [self-hosting tools](/docs/ts/self-host/build) or with the optional [Cloud Platform](https://encore.dev/use-cases/devops-automation), which fully-automates infrastructure management in your own AWS/GCP account.
- **AI-assisted development**: Encore is built for AI coding assistants. With [Encore-specific rules and MCP integration](/docs/ts/ai-integration), AI understands your architecture and can generate type-safe, pattern-consistent code and introspect your app—services, APIs, databases, and traces.

## High-performance Rust runtime

To enable Encore's functionality in TypeScript, we’ve created a high-performance distributed systems runtime in Rust.
It integrates with the standard Node.js runtime for executing JavaScript code, ensuring **100% compatibility with the Node.js ecosystem**.

It provides a number of benefits over standard Node.js:
- **Handles requests validation, provides API type-safety, has built-in observability, and integrates with databases, Pub/Sub, and more**
- **9x increased throughput and 85% reduced latency** compared to standard Node.js/Express.js [See benchmarks](https://encore.dev/blog/event-loops)
- **Zero NPM dependencies** for improved security and faster builds

### How it works

Encore.ts is designed to let the Node.js event loop — which is single-threaded — focus on executing your business logic, while everything else happens in Encore’s multi-threaded Rust runtime. Here's a high-level overview of how this works:

**1. Node.js starts up and initializes the Encore Rust runtime. The Rust runtime then:**
   - Begins accepting incoming requests
   - Parses and validates these requests against the API schema

**2. For each request, the Encore Runtime:**
   - Passes the request to your application code
   - Waits for your code to process the request
   - Sends the response back to the client

**3. When your application needs to interact with infrastructure (like databases or PubSub):**
   - It delegates these tasks to the Rust runtime
   - The Rust runtime handles these operations more efficiently than Node.js would, providing faster execution and lower latency

## Enhanced type-safety for distributed systems

Encore leverages static code analysis to parse the API schema and TypeScript types you define. This enables a number of features:
- Built-in [local development dashboard](/docs/ts/observability/dev-dash)
- API Explorer, automatic documentation, and local tracing
- Runtime type-safety, automatically validating incoming requests against the API schema
- Eliminating runtime errors due to missing required fields

## No DevOps experience required

Encore provides open source tools to help you integrate with your cloud infrastructure, enabling you to self-host your application anywhere that supports Docker containers.
Learn more in the [self-host documentation](/docs/ts/self-host/build).

You can also use [Encore Cloud](https://encore.dev/use-cases/devops-automation), which fully automates provisioning and managing infrastructure in your own cloud on AWS and GCP.

This approach dramatically reduces the level of DevOps expertise required to use scalable, production-ready, cloud services like Kubernetes and Pub/Sub. And because your application code is the source of truth for infrastructure requirements, it ensures the infrastructure in all environments is always in sync with the application's current requirements.

## Simplicity without giving up flexibility

Encore.ts provides integrations for common infrastructure primitives, but also allows for flexibility.

For example, you can always use any cloud infrastructure, even if it's not built into the Encore.ts framework. You can use any database, message broker, or other service that your application needs, just set up the infrastructure and then reference it in your code as you would do traditionally.

If you use [Encore Cloud](https://encore.dev/use-cases/devops-automation), it will [automate infrastructure](/docs/platform/infrastructure/infra) using your own cloud account, so you always have full access to your services from the cloud provider's console.
