---
seotitle: Introduction to Encore – the Backend Development Platform
seodesc: Learn how Encore works and how it helps backend developers build cloud based backend applications without manually dealing with infrastructure.
title: What is Encore?
subtitle: A Development Platform for cloud backend applications
---

Cloud services enable us to build highly scalable applications, but offer a poor developer experience. They force us to deal with a lot of complexity and commonly introduce repetitive work that steals time away from the real goal of building your product. Launching a new app, migrating to the cloud, or breaking apart a monolith into microservices, can therefore be a daunting task.

Encore is designed to solve this problem, restoring creativity for developers and productivity for teams.

Encore gives you a complete toolset for application development, cloud infrastructure management, and production monitoring, all while eliminating the need for many repetitive manual tasks.

## Simplified workflow with Encore's Infrastructure SDK

<img className="w-full h-auto noshadow" src="/assets/docs/encore_overview.png" title="Encore Overview" />

Encore's functionality is based on a [declarative Infrastructure SDK](/docs/primitives/overview) which lets you define resources like services, databases, and queues, as type-safe objects within your application code. 

When you run your app, Encore parses your code and automatically sets up the corresponding infrastructure, seamlessly adapting to local, preview, and cloud environments. This removes the need to manage specific services or configurations during development. _No more messing around with Docker Compose!_

For production, select the cloud provider you want and [connect your cloud account](/docs/deploy/own-cloud). At deployment Encore automatically provisions your [infrastructure](/docs/deploy/infra) using pre-built solutions for popular and battle-tested cloud services on AWS or GCP, such as Cloud Run, Fargate, Kubernetes, CloudSQL, RDS, PubSub, Redis, Cron Jobs, and more.

This means you can focus on product development and avoid dealing with Terraform or other manual infrastructure configuration. This approach also lets you modify and swap out your infrastructure over time, without needing to make code changes or manually update infrastructure config files.

## Built-in tools for development and operations

Our goal is that with Encore, you can focus your engineering effort on your product and leave the platform work to Encore.

For this reason, Encore provides a complete toolset for both developers and DevOps out-of-the-box, including [logging](/docs/observability/logging), [metrics](/docs/observability/metrics), [distributed tracing](/docs/observability/tracing), generated [architecture diagrams](/docs/observability/encore-flow) and [API documentation](/docs/develop/api-docs), [frontend clients](/docs/develop/client-generation), and more.

This is a powerful resource for teams that want to avoid investing engineering time in building out *yet another developer platform*. For teams that already have tools in place, Encore provides third-party integrations with common tools like GitHub, Grafana, Datadog, and more.

## Why choose Encore?

- **Faster Development**: Encore streamlines the development process with its infrastructure SDK, clear abstractions, and built-in development tools, enabling you to build and deploy applications more quickly.
- **Reduced Costs**: Encore's infrastructure management minimizes wasteful cloud expenses and reduces DevOps workload, allowing you to work more efficiently.
- **Scalability & Performance**: Encore simplifies building microservices applications that can handle growing user bases and demands, without the normal boilerplate and complexity.
- **Control & Standardization**: Encore enforces standardization and provisions infrastructure consistently according to best practises for each cloud provider.
- **Observability:** Built-in tools like automated architecture diagrams and API docs, infrastructure tracking, and distributed tracing make it simple for teams and leaders to get an always up-to-date picture of the system.
- **Security**: Encore ensures your application is secure by automating IAM management and implementing _principle of least privilege_ security by default.

## Common use cases

Encore is designed to give teams a productive and less complex experience when solving most backend use cases. Many teams use Encore to build things like:

-   High-performance B2B Platforms
-   Fintech & Consumer apps
-   Global E-commerce marketplaces
-   Microservices backends for SaaS applications and mobile apps
-   And much more...

## Getting started

1. [Sign up and install the Encore CLI](https://encore.dev/signup)
2. [Follow a tutorial and start building](https://encore.dev/docs/tutorials/)
3. [Book a 1:1](https://encore.dev/book) or [join Slack](https://encore.dev/slack) to discuss your use case or how to begin adopting Encore
4. Follow and star the project on [GitHub](https://github.com/encoredev/encore) to stay up to date
5. Explore the Documentation to learn about Encore's features

_...or keep reading to learn more about how Encore works._

## Meet the Encore application model

Encore works by using static analysis to understand your application. This is a fancy term for parsing and analyzing the code you write and creating a graph of how your application works. This graph closely represents your mental model of your system: boxes and arrows, representing systems and services that communicate with other systems, pass data and connect to infrastructure. We call it the Encore Application Model.

Because Encore's Infrastructure SDK, parser, and compiler, are all designed together, Encore can ensure 100% accuracy when creating the application model. Any deviation is caught as a compilation error.

Using this model, Encore can provide tools to solve problems that normally would be up to the developer to do manually. From creating architecture diagrams and API documentation to provisioning cloud infrastructure.

We're continuously expanding on Encore's capabilities and are building a new generation of developer tools that are enabled by Encore's understanding of your application.

The infrastructure SDK, parser, and compiler that enable this are all [Open Source](https://github.com/encoredev/encore).

<img src="/assets/docs/flow-diagram.png" title="Encore Application Model" className="mx-auto md:max-w-lg"/>

## Standardization brings clarity

Developers make dozens of decisions when creating a backend application. Deciding how to structure the codebase, defining API schemas, picking underlying infrastructure, etc. The decisions often come down to personal preferences, not technical rationale. This creates a huge problem in the form of fragmentation! When every stack looks different, all tools have to be general purpose.

When you adopt Encore, many of these stylistic decisions are already made for you. Encore's [Infrastructure SDK](/docs/primitives) ensures your application follows modern best practices. And when you run your application, Encore's Open Source parser and compiler check that you're sticking to the standard. This means you're free to focus your energy on what matters: writing your application's business logic.
