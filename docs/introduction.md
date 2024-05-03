---
seotitle: Introduction to Encore – the Backend Development Platform
seodesc: Learn how Encore works and how it helps backend developers build cloud-based backend applications without manually dealing with infrastructure.
title: What is Encore?
subtitle: A Development Platform for cloud backend applications
---

Cloud services enable us to build highly scalable applications, but offer a poor developer experience. They force developers to manage a lot of added complexity during development and commonly introduce repetitive work that steals time away from the real goal of building your product. Launching a new app, migrating to the cloud, or breaking apart a monolith into microservices, can therefore be a daunting task.

Encore is purpose-built to solve this problem, restoring creativity for developers and productivity for teams.

## A simplified cloud backend development workflow

Encore provides a complete toolset for backend development, from local development and testing, to cloud infrastructure management and DevOps.

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_full.png" title="Encore Overview" />

Much of Encore's functionality is enabled by the Open Source [declarative Infrastructure SDK](/docs/primitives/overview), which lets you define resources like services, databases, cron jobs, and Pub/Sub, as type-safe objects in your application code.

With the SDK you only define **infrastructure semantics** — _the things that matter to your application's behavior_ — not configuration for _specific_ cloud services. Encore then automatically generates boilerplate and orchestrates the relevant infrastructure for each environment. This means your application code can be used to run locally, test in preview environments, and provision and deploy to cloud environments on AWS and GCP. 

When your application is deployed to your cloud, there are **no runtime dependencies on Encore** and there is **no proprietary code running in your cloud**.

## Local Development

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_local.png" title="Encore's Local Development Workflow" />

When you run your app locally using the [Encore CLI](/docs/install), Encore parses your code and automatically sets up the necessary local infrastructure on the fly. _No more messing around with Docker Compose!_

Aside from managing infrastructure, Encore's local development workflow comes with a lot of tools to make building distributed systems easier:

- **Local environment matches cloud:** Encore automatically handles the semantics of service communication and interfacing with different types of infrastructure services, so that the local environment is a 1:1 representation of your cloud environment.
- **Cross-service type-safety:** When building microservices applications with Encore, you get type-safety and auto-complete in your IDE when making cross-service API calls.
- **Type-aware infrastructure:** With Encore, infrastructure like Pub/Sub queues are type-aware objects in your program. This enables full end-to-end type-safety when building event-driven applications.
- **Secrets management:** Built-in [secrets management](/docs/primitives/secrets) for all environments.
- **Tracing:** The [local development dashboard](/docs/observability/dev-dash) provides local tracing to help understand application behavior and find bugs.
- **Automatic API docs & clients:** Encore generates [API docs](/docs/develop/api-docs) and [API clients](/docs/develop/client-generation) in Go, TypeScript, JavaScript, and OpenAPI specification.

## Testing

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_testing.png" title="Encore's Testing Workflow" />

Encore comes with several built-in tools to help with testing:

- **Built-in service/API mocking:** Encore provides built-in support for [mocking API calls](/docs/develop/testing/mocking), and interfaces for automatically generating mock objects for your services.
- **Local test infra:** When running tests locally, Encore automatically provides dedicated [test infrastructure](/docs/develop/testing#test-only-infrastructure) to isolate individual tests.
- **Local test tracing:** The [local dev dashboard](/docs/observability/dev-dash) provides distributed tracing for tests, providing great visibility into what's happening and making it easier to understand why a test failed.
- **Preview Environments:** Encore automatically provisions a [Preview Environment](/docs/deploy/preview-environments) for each Pull Request, an effective tool when doing end-to-end testing.

## DevOps

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_devops.png" title="Encore's DevOps Workflow" />

Our goal is that when you use Encore, you can focus your engineering effort on your product and completely avoid investing time in building a developer platform. You also get built-in tools that automate >90% of the normal day-to-day DevOps work.

To achieve this, the headline feature Encore provides is automatic infrastructure provisioning in your cloud. Instead of writing Terraform, YAML, or clicking in cloud consoles, you [connect your cloud account](/docs/deploy/own-cloud) and hit deploy. At deployment Encore automatically provisions [infrastructure](/docs/deploy/infra) using battle-tested cloud services on AWS or GCP. Such as Cloud Run, Fargate, Kubernetes, CloudSQL, RDS, Pub/Sub, Redis, Cron Jobs, and more.

This is enabled by Encore's Open-Source [Infrastructure SDK](/docs/primitives/overview), which lets you declare infrastructure semantics in application code. This approach lets you modify and swap out your infrastructure over time, without needing to make code changes or manually update infrastructure config files.

Here are some of the other benefits and DevOps tools provided by Encore:

- **No IaC or YAML needed:** Encore removes the need for manual infrastructure configuration, the application code is the source of truth for both business logic and infrastructure semantics.
- **Automatic least-privilege IAM:** Encore parses your application code and sets up least-privilege IAM to match the requirements of the application.
- **Infra tracking & approvals workflow:** Encore keeps track of all the [infrastructure](/docs/deploy/infra) it provisions and provides an approval workflow as part of the deployment process, so Admins can verify and approve all infra changes.
- **Cloud config 2-way sync:** Encore provides [a simple UI to make configuration changes](/docs/deploy/infra#configurability), and also supports syncing changes you make in your cloud console on AWS/GCP.
- **Cost analytics:** A simple overview to monitor costs for all infrastructure provisioned by Encore in your cloud.
- **Logging & Metrics:** Encore automatically provides [logging](/docs/observability/logging), [metrics](/docs/observability/metrics), and [integrates with 3rd party tools](/docs/observability/metrics#integrations-with-third-party-observability-services) like Datadog and Grafana.
- **Service Catalog:**  Encore automatically generates a service catalog with complete [API documentation](/docs/develop/api-docs).
- **Architecture diagrams:** To help with onboarding and collaboration, Encore generates [architecture diagrams](/docs/observability/encore-flow) for your application, including infrastructure dependencies.

## Why choose Encore?

We believe Encore's end-to-end workflow is an unfair advantage for teams that want to focus on their product, and avoid investing engineering time in building *yet another developer platform*.

Encore is designed to provide engineering teams with all the tools they need to build production-ready cloud backends:

- **Faster Development**: Encore streamlines the development process with its infrastructure SDK, clear abstractions, and built-in development tools, enabling you to build and deploy applications more quickly.
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

See the [users stories](/customers) section for more on how teams are using Encore to power their development.

## Getting started

1. [Sign up and install the Encore CLI](https://encore.dev/signup)
2. [Follow a tutorial and start building](https://encore.dev/docs/tutorials/)
3. [Book a 1:1](https://encore.dev/book) or [join Discord](https://encore.dev/discord) to discuss your use case or how to begin adopting Encore
4. Follow and star the project on [GitHub](https://github.com/encoredev/encore) to stay up to date
5. Explore the Documentation to learn about Encore's features

_...or keep reading to learn more about how Encore works._

## Meet the Encore application model

Encore works by using static analysis to understand your application. This is a fancy term for parsing and analyzing the code you write and creating a graph of how your application works. This graph closely represents your own mental model of the system: boxes and arrows that represent systems and services that communicate with other systems, pass data and connect to infrastructure. We call it the Encore Application Model.

Because Encore's Open Source Infrastructure SDK, parser, and compiler, are all designed together, Encore can ensure 100% accuracy when creating the application model. Any deviation is caught as a compilation error.

Using this model, Encore can provide tools to solve problems that normally would be up to the developer to do manually. From creating architecture diagrams and API documentation to provisioning cloud infrastructure.

We're continuously expanding on Encore's capabilities and are building a new generation of developer tools that are enabled by Encore's understanding of your application.

The infrastructure SDK, parser, and compiler that enable this are all [Open Source](https://github.com/encoredev/encore).

<img src="/assets/docs/flow-diagram.png" title="Encore Application Model" className="mx-auto md:max-w-lg"/>

## Standardization brings clarity

Developers make dozens of decisions when creating a backend application. Deciding how to structure the codebase, defining API schemas, picking underlying infrastructure, etc. The decisions often come down to personal preferences, not technical rationale. This creates a huge problem in the form of fragmentation! When every stack looks different, all tools have to be general purpose.

When you adopt Encore, many of these stylistic decisions are already made for you. Encore's [Infrastructure SDK](/docs/primitives) ensures your application follows modern best practices. And when you run your application, Encore's Open Source parser and compiler check that you're sticking to the standard. This means you're free to focus your energy on what matters: writing your application's business logic.
