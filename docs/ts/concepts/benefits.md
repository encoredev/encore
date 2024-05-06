---
seotitle: Benefits of using Encore for TypeScript
seodesc: Get to know the benefits of using Encore's Backend SDK for TypeScript to build cloud-native backend applications.
title: Benefits of using Encore for TypeScript
toc: false
lang: ts
---

## Integrated developer experience for empowering productivity

- **Develop new features locally as if the infrastructure is already set up**: Encore automatically compiles your app and sets up the necessary infrastructure on the fly.
- **Faster feedback loops:** With type-safe infrastructure you can identify problems as early as in your code editor, instead of learning about issues via the — much slower — deployment cycle.
- **No manual config required**: There is no need to manually write [IaC](/resources/infrastructure-as-code) configuration, like Terraform, and no need to maintain configuration for multiple environments manually. Encore uses your application code as the source of truth and automatically keeps all environments in sync.
- **One codebase for all environments**: Encore [automatically provisions](/docs/deploy/infra) your local, [preview](/docs/deploy/preview-environments), and cloud environments (using [your own cloud account](/docs/deploy/own-cloud)) on AWS/GCP.
- **Cloud-agnostic by default**: The Backend SDK is an abstraction layer on top of the cloud provider's APIs, so you avoid becoming locked in to a single cloud.
- **Evolve infrastructure without code changes**: As your requirements evolve, you can change and configure the provisioned infrastructure by using Encore's Cloud Dashboard or your cloud provider's console.
  
## Incredible performance powered by a custom Rust runtime

To enable Encore's functionality in TypeScript, we’ve created a high-performance distributed systems runtime in Rust.
It integrates with the standard Node.js runtime for excecuting JavaScript code, ensuring **100% compatability with the Node.js ecosystem**.

The Rust runtime does everything from handling incoming requests and making API calls, to querying databases and using Pub/Sub.
It even handles all application observability, like distributed tracing, structured logging, and metrics.

Using Rust leads to a **massive performance increase** over standard Node.js, **increasing throughput by 7x** and **reducing response latency by 85%**.

What’s really cool: **Encore has zero NPM dependencies**, improving security and speeding up builds and application startup times.

### How it works

1. Node.js starts up, and initializes the Encore Rust runtime, which begins accepting incoming requests, parsing and validating them against the API schema.
2. The Encore Runtime then passes on the request to your application code, and waits for the response, before sending it back out over the wire.
3. When your application uses infrastructure resources, like querying databases or publishing PubSub messages, it hands that over to the Rust runtime for faster execution.

This means that the Node.js event loop — which is single-threaded — can focus on executing your business logic. Everything else happens in Encore’s multi-threaded Rust runtime.

## Enhanced type-safety for distributed systems

Encore uses static code analysis to parse and analyze the TypeScript types you define.
This powers Encore’s built-in [local development dashboard](/docs/observability/dev-dash), which provides an API Explorer for making API calls, automatic API documentation, local tracing, and much more.

Normally with TypeScript, the type information is lost at runtime. But Encore is different.
Encore uses the API schema to automatically validate incoming requests, guaranteeing complete type-safety, even at runtime.
No more confusing exceptions because a required field is missing.

## No DevOps experience required

Because Encore orchestrates setting up and configuring cloud services in your cloud on AWS or GCP, it dramatically reduces the level of DevOps expertise required to use scalable, production-ready, cloud services like Kubernetes and Pub/Sub. And because your application code is the source of truth for infrastructure requirements, it ensures the infrastructure in all your environments are always in sync with the application's requirements.

## Simplicity without giving up flexibility

While most requirements are met by a common set of infrastructure primitives, sooner or later you will likely need something highly specific to your problem domain. Encore is designed to ensure you can use any cloud infrastructure, even if it's not built into Encore's Backend SDK. This works seamlessly since Encore [provisions infrastructure](/docs/deploy/infra) in your own cloud account, so you can use any of your cloud provider's services as you traditionally would.
