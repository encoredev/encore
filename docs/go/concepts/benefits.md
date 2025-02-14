---
seotitle: Benefits of using Encore.go
seodesc: See how Encore.go helps you build backends faster using Go.
title: Encore.go Benefits
subtitle: How Encore.go helps you build robust distributed systems, faster.
lang: go
---

Using Encore.go to declare infrastructure in application code helps unlock several benefits:

- **Local development with instant infrastructure**: Encore.go automatically sets up necessary infrastructure as you develop.
- **Rapid feedback**: Catch issues early with type-safe infrastructure, avoiding slow deployment cycles.
- **No manual configuration required**: No need for Infrastructure-as-Code. Your code is the single source of truth.
- **Unified codebase**: One codebase for all environments; local, preview, and cloud.
- **Cloud-agnostic by default**: Encore.go provides an abstraction layer on top of the cloud provider's APIs, so you avoid becoming locked in to a single cloud.
- **Evolve infrastructure without code changes**: As requirements evolve, you can change the provisioned infrastructure without making code changes, you only need to change the infrastructure configuration which is separate from the application code.

## No DevOps experience required

Encore provides open source tools to help you integrate with your cloud infrastructure, enabling you to self-host your application anywhere to supports Docker containers.
Learn more in the [self-host documentation](/docs/go/self-host/docker-build).

You can also use [Encore Cloud](https://encore.dev/use-cases/devops-automation), which fully automates provisioning and managing infrastructure in your own cloud on AWS and GCP.

This approach dramatically reduces the level of DevOps expertise required to use scalable, production-ready, cloud services like Kubernetes and Pub/Sub. And because your application code is the source of truth for infrastructure requirements, it ensures the infrastructure in all your environments are always in sync with the application's requirements.

## Simplicity without giving up flexibility

Encore.go provides integrations for common infrastructure primitives, but also allows for flexibility. You can always use any cloud infrastructure, even if it's not built into Encore.go. If you use Encore's [Cloud Platform](https://encore.dev/use-cases/devops-automation), it [automates infrastructure](/docs/platform/infrastructure/infra) using your own cloud account, so you always have full access to your services from the cloud provider's console.
