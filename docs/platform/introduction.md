---
seotitle: Introduction to Encore Cloud Platform
seodesc: Learn how Encore works and how it helps backend developers build cloud-based backend applications without manually dealing with infrastructure.
title: Encore Cloud Platform
subtitle: End-to-end development platform for building robust distributed systems
lang: platform
---

Cloud services enable us to build highly scalable applications, but offer a poor developer experience. They force developers to manage a lot of added complexity during development and commonly introduce repetitive work that steals time away from the real goal of building your product. Launching a new app, migrating to the cloud, or breaking apart a monolith into microservices, can therefore be a daunting task.

Encore is purpose-built to solve this problem, restoring creativity for developers and productivity for teams.

## Intro video

<iframe width="360" height="202" src="https://www.youtube.com/embed/vvqTGfoXVsw?si=TliVv2VAT0YtNuYk" title="Encore Intro Video" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe>

## A simplified cloud backend development workflow

Encore provides a complete toolset for backend development, from local development and testing, to cloud infrastructure management and DevOps.

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_full.png" title="Encore Overview" />

Much of Encore's functionality is enabled by the Open Source Backend Frameworks [Encore.ts](/docs/ts) and [Encore.go](/docs/go), which let you define resources like services, databases, cron jobs, and Pub/Sub, as type-safe objects in your application code.

With the Backend Framework you only define **infrastructure semantics** — _the things that matter to your application's behavior_ — not configuration for _specific_ cloud services. Encore then automatically generates boilerplate and orchestrates the relevant infrastructure for each environment. This means your application code can be used to run locally, test in preview environments, and provision and deploy to cloud environments on AWS and GCP. 

When your application is deployed to your cloud, there are **no runtime dependencies on Encore** and there is **no proprietary code running in your cloud**.

## Local Development

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_local.png" title="Encore's Local Development Workflow" />

When you run your app locally using the [Encore CLI](/docs/ts/install), Encore parses your code and automatically sets up the necessary local infrastructure on the fly. _No more messing around with Docker Compose!_

Aside from managing infrastructure, Encore's local development workflow comes with a lot of tools to make building distributed systems easier:

- **Local environment matches cloud:** Encore automatically handles the semantics of service communication and interfacing with different types of infrastructure services, so that the local environment is a 1:1 representation of your cloud environment.
- **Cross-service type-safety:** When building microservices applications with Encore, you get type-safety and auto-complete in your IDE when making cross-service API calls.
- **Type-aware infrastructure:** With Encore, infrastructure like Pub/Sub queues are type-aware objects in your program. This enables full end-to-end type-safety when building event-driven applications.
- **Secrets management:** Built-in [secrets management](/docs/ts/primitives/secrets) for all environments.
- **Tracing:** The [local development dashboard](/docs/ts/observability/dev-dash) provides local tracing to help understand application behavior and find bugs.
- **Automatic API docs & clients:** Encore generates [API docs](/docs/ts/observability/service-catalog) and [API clients](/docs/ts/cli/client-generation) in Go, TypeScript, JavaScript, and OpenAPI specification.

## Testing

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_testing.png" title="Encore's Testing Workflow" />

Encore comes with several built-in tools to help with testing:

- **Built-in service/API mocking:** Encore provides built-in support for [mocking API calls](/docs/go/develop/testing/mocking), and interfaces for automatically generating mock objects for your services.
- **Local test infra:** When running tests locally, Encore automatically provides dedicated [test infrastructure](/docs/go/develop/testing#test-only-infrastructure) to isolate individual tests.
- **Local test tracing:** The [local dev dashboard](/docs/go/observability/dev-dash) provides distributed tracing for tests, providing great visibility into what's happening and making it easier to understand why a test failed.
- **Preview Environments:** Encore automatically provisions a [Preview Environment](/docs/platform/deploy/preview-environments) for each Pull Request, an effective tool when doing end-to-end testing.

## DevOps

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_devops.png" title="Encore's DevOps Workflow" />

Our goal is that when you use Encore, you can focus your engineering effort on your product and completely avoid investing time in building a developer platform. You also get built-in tools that automate >90% of the normal day-to-day DevOps work.

To achieve this, the headline feature Encore provides is automatic infrastructure provisioning in your cloud. Instead of writing Terraform, YAML, or clicking in cloud consoles, you [connect your cloud account](/docs/platform/infrastructure/own-cloud) and hit deploy. At deployment Encore automatically provisions [infrastructure](/docs/platform/infrastructure/infra) using battle-tested cloud services on AWS or GCP. Such as Cloud Run, Fargate, Kubernetes, CloudSQL, RDS, Pub/Sub, Redis, Cron Jobs, and more.

This is enabled by [Encore.ts](/docs/ts) and [Encore.go](/docs/go), which lets you declare infrastructure semantics in application code. This approach lets you modify and swap out your infrastructure over time, without needing to make code changes or manually update infrastructure config files.

Here are some of the other benefits and DevOps tools provided by Encore:

- **No IaC or YAML needed:** Encore removes the need for manual infrastructure configuration, the application code is the source of truth for both business logic and infrastructure semantics.
- **Automatic least-privilege IAM:** Encore parses your application code and sets up least-privilege IAM to match the requirements of the application.
- **Infra tracking & approvals workflow:** Encore keeps track of all the [infrastructure](/docs/platform/infrastructure/infra) it provisions and provides an approval workflow as part of the deployment process, so Admins can verify and approve all infra changes.
- **Cloud config 2-way sync:** Encore provides [a simple UI to make configuration changes](/docs/platform/infrastructure/infra#configurability), and also supports syncing changes you make in your cloud console on AWS/GCP.
- **Cost analytics:** A simple overview to monitor costs for all infrastructure provisioned by Encore in your cloud.
- **Logging & Metrics:** Encore automatically provides [logging](/docs/ts/observability/logging), [metrics](/docs/platform/observability/metrics), and [integrates with 3rd party tools](/docs/platform/observability/metrics#integrations-with-third-party-observability-services) like Datadog and Grafana.
- **Service Catalog:**  Encore automatically generates a service catalog with complete [API documentation](/docs/ts/observability/service-catalog).
- **Architecture diagrams:** To help with onboarding and collaboration, Encore generates [architecture diagrams](/docs/ts/observability/encore-flow) for your application, including infrastructure dependencies.

## Why choose Encore?

We believe Encore's end-to-end workflow is an unfair advantage for teams that want to focus on their product, and avoid investing engineering time in building *yet another developer platform*.

Encore is designed to provide engineering teams with all the tools they need to build production-ready cloud backends:

- **Faster Development**: Encore streamlines the development process with its Backend Framework, clear abstractions, and built-in development tools, enabling you to build and deploy applications more quickly.
- **Reduced Costs**: Encore's infrastructure management minimizes wasteful cloud expenses and reduces DevOps workload, allowing you to work more efficiently.
- **Scalability & Performance**: Encore simplifies building microservices applications that can handle growing user bases and demands, without the normal boilerplate and complexity.
- **Control & Standardization**: Encore enforces standardization and provisions infrastructure consistently according to best practices for each cloud provider.
- **Quality through understandability:** Built-in tools like automated architecture diagrams, generated API docs, and distributed tracing make it simple for teams to get an overview of their system and understand its behavior.
- **Security**: Encore ensures your application is secure by implementing cloud security best practices and _principle of least privilege_ security by default.

## Common use cases

Encore is designed to give teams a productive and less complex experience when solving most backend use cases. Many teams use Encore to build things like:

-   High-performance B2B Platforms
-   Fintech & Consumer apps
-   Global E-commerce marketplaces
-   Microservices backends and event-driven systems for SaaS applications and mobile apps
-   And much more...

Check out the [showcase](/showcase) section for some examples of real-world products being built with Encore.

## Getting started

1. [Follow the Quick Start Guide](/docs/ts/quick-start)
2. [Join Discord](https://encore.dev/discord) to ask questions and meet other Encore developers
3. Follow and star the project on [GitHub](https://github.com/encoredev/encore) to stay up to date
4. Explore the Documentation to learn about Encore's features
