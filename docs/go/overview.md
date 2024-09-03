---
seotitle: Start building backends using Encore.go
seodesc: Learn how Encore's Go Backend Framework works, and get to know the powerful features that help you build cloud backend applications easier than ever before.
title: Encore.go Backend Framework
subtitle: Learn how to use Encore.go to build scalable backend applications and distributed systems
toc: false
lang: go
---

Encore.go simplifies backend development by providing an application-centric way of using most common infrastructure primitives, things like: databases, queues, cron jobs, and APIs.
It lets you define these primitives as type-safe objects directly in your application code. This approach streamlines your development process in several ways:

1. Local Environment Management: Encore.go automatically handles the setup and running of your local development environments.

2. Enhanced Observability: It provides [built-in tools](/docs/observability/dev-dash) for monitoring and understanding your application's behavior.

3. Automatic Documentation: Encore.go generates and maintains [up-to-date documentation](/docs/develop/api-docs) for your APIs and services.

4. Cloud Integration: It simplifies and facilitates [integrating with cloud infrastructure](/docs/how-to/self-host), making deployment and scaling easier.

5. DevOps Automation: For those seeking a fully automated solution, Encore offers an optional [Cloud Platform](/use-cases/devops-automation) that automates infrastructure provisioning, IAM management, and DevOps processes on AWS and GCP.

By combining these features, Encore.go enables you to focus on writing your application logic while it automates much of the complexities of modern backend development.


### See how to use each primitive

- [Services and APIs](/docs/primitives/services-and-apis)
- [Databases](/docs/primitives/databases)
- [Cron Jobs](/docs/primitives/cron-jobs)
- [Pub/Sub & Queues](/docs/primitives/pubsub)
- [Caching](/docs/primitives/caching)
- [Secrets](/docs/primitives/secrets)

### Tutorials

Check out the [Tutorials](/docs/tutorials) section for examples of complete Encore applications.

<img src="/assets/docs/primitives.png" title="Cloud Primitives" className="noshadow mx-auto d:w-1/2"/>

## Benefits of using Encore.go

Using Encore.go to declare infrastructure in application code helps unlock these benefits:

- **Local development with instant infrastructure**: Encore.go automatically sets up necessary infrastructure as you develop.
- **Rapid feedback**: Catch issues early with type-safe infrastructure, avoiding slow deployment cycles.
- **No manual configuration required**: No need for Infrastrucutre-as-Code. Your code is the single source of truth.
- **Unified codebase**: One codebase for all environments; local, preview, and cloud.
- **Cloud-agnostic by default**: Encore.go provides an abstraction layer on top of the cloud provider's APIs, so you avoid becoming locked in to a single cloud.
- **Evolve infrastructure without code changes**: As requirements evolve, you can change the provisioned infrastructure without making code changes, either using the Open Source [self-hosting tools](/docs/deploy/self-hosting) or fully-automated in your AWS/GCP account using [Encore's Cloud Platform](https://encore.dev/use-cases/devops-automation).
cloud provider's console.


## No DevOps experience required

Encore provides open source tools to help you integrate with your cloud infrastructure, enabling you to self-host your application anywhere to supports Docker containers.
Learn more in the [self-host documentation](/docs/deploy/self-hosting).

You can also use [Encore's Cloud Platform](https://encore.dev/use-cases/devops-automation), which fully automates provisioning and managing infrastructure in your own cloud on AWS and GCP.

This approach dramatically reduces the level of DevOps expertise required to use scalable, production-ready, cloud services like Kubernetes and Pub/Sub. And because your application code is the source of truth for infrastructure requirements, it ensures the infrastructure in all your environments are always in sync with the application's requirements.

## Simplicity without giving up flexibility

Encore.go provides integrations for common infrastructure primitives, but also allows for flexibility. You can always use any cloud infrastructure, even if it's not built into Encore.go. If you use Encore's [Cloud Platform](https://encore.dev/use-cases/devops-automation), it [automates infrastructure](/docs/deploy/infra) using your own cloud account, so you always have full access to your services from the cloud provider's console.
