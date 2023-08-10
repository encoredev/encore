---
seotitle: Encore's Infrastructure SDK provides the common backend building blocks
seodesc: Learn how to build cloud-agnostic backend applications using Encore's built-in cloud primitives.
title: Infrastructure SDK
subtitle: Providing a declarative way of using common infrastructure primitives
---

Modern backend applications rely on a common set of infrastructure primitives for most of the behavior. To improve your development workflow, Encore's Infrastructure SDK provides a declarative way of using them directly in application code. This comes with several benefits:
- **Develop new features locally as if the infrastructure is already set up**: Encore automatically compiles your app and sets up the necessary infrastructure on the fly.
- **No manual maintenance required**: There is no need to manually write [IaC](/resources/infrastructure-as-code) configuration, like Terraform, and no need to manually maintain configuration for multiple environments. Encore uses your application code as the single source of truth and automatically keeps all environments in sync.
- **One codebase for all environments**: Encore [automatically provisions](/docs/deploy/infra) your local, [preview](/docs/deploy/preview-environments), and cloud environments (using [your own cloud accunt](/docs/deploy/own-cloud)).
- **Cloud-agnostic by default**: The Infrastructure SDK is an abstraction layer on top of the cloud provider's APIs, so you avoid becoming locked-in to a single cloud.
- **Evolve infrastructure without code changes**: As your requirements evolve, you can change and configure the provisioned infrastructure by using Encore's Cloud Dashboard or your cloud provider's console.

See how to use each primitive:

- [Services and APIs](/docs/primitives/services-and-apis)
- [Databases](/docs/primitives/databases)
- [Cron Jobs](/docs/primitives/cron-jobs)
- [Pub/Sub & Queues](/docs/primitives/pubsub)
- [Caching](/docs/primitives/caching)
- [Secrets](/docs/primitives/secrets)

<img src="/assets/docs/primitives.png" title="Cloud Primitives" className="noshadow mx-auto d:w-1/2"/>

## Simplicity without giving up flexibility

While most requirements are met by a common set of infrastructure primitives, sooner or later you will likely need something highly specific to your problem domain. Encore is designed to ensure you can use any type of cloud infrastructure, even if it's not built into Encore's Infrastructure SDK. This works seamlessly since Encore [provisions infrastructure](/docs/deploy/infra) in your own cloud account, so you can use any of your cloud provider's services as you traditionally would.
