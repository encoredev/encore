---
seotitle: Introduction to Encore Cloud
seodesc: Learn how Encore Cloud works and how it helps backend developers build cloud-based backend applications without manually dealing with infrastructure.
title: Encore Cloud
subtitle: End-to-end development platform for building robust distributed systems
lang: platform
---

While cloud services enable us to build powerful applications, they come with significant complexity.

Developers spend countless hours managing infrastructure, writing boilerplate code, and dealing with complex deployment processes instead of building features that matter to users.

Launching a new app, migrating to the cloud, or breaking apart a monolith into microservices, can therefore be a daunting task.
Encore Cloud is purpose-built to solve this problem, restoring creativity for developers and productivity for teams.

## Intro video

<iframe width="360" height="202" src="https://www.youtube.com/embed/vvqTGfoXVsw?si=TliVv2VAT0YtNuYk" title="Encore Intro Video" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe>

## A simplified cloud backend development workflow

Encore Cloud provides a complete toolset for backend development: from local development, testing, and observability, to cloud infrastructure and DevOps automation.

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_full.png" title="Encore Overview" />

Much of Encore Cloud's functionality is enabled by Encore's open source backend frameworks, [Encore.ts](/docs/ts) and [Encore.go](/docs/go), which let you define resources like services, databases, cron jobs, and Pub/Sub, as type-safe objects in your application code.

With the backend frameworks you only define **infrastructure semantics** — _the things that matter to your application's behavior_ — not configuration for _specific_ cloud services. Encore Cloud then automatically generates boilerplate and orchestrates the relevant infrastructure for each environment. This means your application code can be used to run locally, test in preview environments, and provision and deploy to cloud environments on AWS and GCP.

When your application is deployed to your cloud, there are **no runtime dependencies on Encore Cloud** and there is **no proprietary code running in your cloud**.

## Local Development

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_local.png" title="Encore's Local Development Workflow" />

The local development tooling is fully open source. When you run your app locally using the [Encore CLI](/docs/ts/install), it parses your code and automatically sets up the necessary local infrastructure on the fly. _No more messing around with Docker Compose!_

Aside from managing infrastructure, Encore's local development workflow comes with a lot of tools to make building distributed systems easier:

- **Local environment matches cloud:** Encore automatically handles the semantics of service communication and interfacing with different types of infrastructure services, so that the local environment is a 1:1 representation of your cloud environment.
- **Cross-service type-safety:** When building microservices applications with Encore, you get type-safety and auto-complete in your IDE when making cross-service API calls.
- **Type-aware infrastructure:** With Encore, infrastructure like Pub/Sub queues are type-aware objects in your program. This enables full end-to-end type-safety when building event-driven applications.
- **Tracing:** The [local development dashboard](/docs/ts/observability/dev-dash) provides local tracing to help understand application behavior and find bugs.
- **Automatic API docs & clients:** Encore generates [API docs](/docs/ts/observability/service-catalog) and [API clients](/docs/ts/cli/client-generation) in Go, TypeScript, JavaScript, and OpenAPI specification.

## Testing

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_testing.png" title="Encore's Testing Workflow" />

Encore's open source framework comes with several built-in tools to help with testing:

- **Built-in service/API mocking:** Encore provides built-in support for [mocking API calls](/docs/go/develop/testing/mocking), and interfaces for automatically generating mock objects for your services.
- **Local test infra:** When running tests locally, Encore automatically provides dedicated [test infrastructure](/docs/go/develop/testing#test-only-infrastructure) to isolate individual tests.
- **Local test tracing:** The [Local Development Dashboard](/docs/go/observability/dev-dash) provides distributed tracing for tests, providing great visibility into what's happening and making it easier to understand why a test failed.

Encore Cloud adds to this tool-set with:
- **Preview Environments:** Encore automatically provisions a [Preview Environment](/docs/platform/deploy/preview-environments) for each Pull Request, an effective tool when doing end-to-end testing.

## Infrastructure & DevOps automation

<img className="noshadow mx-auto d:w-3/4" src="/assets/docs/arch_devops.png" title="Encore's DevOps Workflow" />

Encore Cloud removes the need to build your own developer platform, automating over 90% of typical DevOps tasks. Here's how:

### Infrastructure Management
- **Zero-config deployment:** Connect your repo and cloud account, then deploy and Encore Cloud handles the rest
- **Automatic infrastructure:** Use battle-tested AWS/GCP services (Cloud Run/Fargate, GKE/EKS, CloudSQL/RDS, Pub/Sub / SQS/SNS, etc.) without any manual setup or configuration
- **No IaC required:** Say goodbye to Terraform and YAML - your code is the single source of truth

### Security & Governance
- **Automated IAM:** Least-privilege security permissions generated from parsing your code
- **Infrastructure tracking:** Complete visibility of all provisioned resources
- **Change management:** Built-in approval workflow for infrastructure changes
- **Configuration management:** Simple UI for config changes that automatically 2-way syncs to your cloud

### Monitoring & Observability
- **Cost monitoring:** Track infrastructure costs across your cloud resources (currently for GCP, AWS coming soon)
- **Integrated observability:** Built-in logging, metrics, and tracing
- **Third-party integration:** Works with Datadog, Grafana, and other tools
- **Auto-generated documentation:**
  - Service catalog with complete API docs
  - Live architecture diagrams showing infrastructure dependencies

## Why choose Encore Cloud?

Encore Cloud's end-to-end workflow is an unfair advantage for teams that need to move quickly without sacrificing quality and scalability.
By automating over 90% of the normal day-to-day DevOps work, you can focus on building your product instead of building your own developer platform.

The benefits of Encore Cloud are:

- **Faster Development**: Encore Cloud enables 2-3x faster iterations thanks to the streamlined the development process with its clear abstractions and built-in development tools.
- **Reduced Costs**: Encore Cloud's infrastructure management minimizes wasteful cloud expenses and reduces DevOps workload by 90%.
- **Scalability & Performance**: Encore Cloud simplifies building microservices applications that can handle growing user bases and demands, without the normal boilerplate and complexity.
- **Control & Standardization**: Encore Cloud enforces standardization and provisions infrastructure consistently according to best practices for each cloud provider.
- **Quality through understandability:** Built-in tools like automated architecture diagrams, generated API docs, and distributed tracing make it simple for teams to get an overview of their system and ensure the correct behavior.
- **Security**: Encore Cloud makes your application secure by automatically implementing security best practices for each cloud provider.

## Common use cases

Encore Cloud is designed to give teams a productive and less complex experience when solving most common backend use cases.
Many teams use Encore Cloud to build things like:

-   Consumer apps
-   High-performance B2B Platforms
-   Fintech & Crypto applications
-   Global E-commerce marketplaces
-   Microservices backends and event-driven systems for scalable SaaS applications
-   And much more...

Check out the [showcase](https://encore.cloud/showcase) section for some examples of real-world products being built with Encore.

## Getting started

1. [Follow the Quick Start Guide](/docs/ts/quick-start)
2. [Join Discord](https://encore.dev/discord) to ask questions and meet other Encore developers
3. Follow and star the project on [GitHub](https://github.com/encoredev/encore) to stay up to date
