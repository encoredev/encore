---
seotitle: Start building backends using Encore for Go
seodesc: Learn how Encore's Go Infrastructure SDK works, and get to know the powerful features that help you build cloud backend applications easier than ever before.
title: Go SDK
subtitle: Learn how to use Encore's Go SDK to build scalable backend applications and distributed systems
toc: false
lang: go
---

Modern backend applications rely on a common set of infrastructure primitives for most of the behavior. To improve your development workflow, Encore's Go SDK lets you declare these primitives as type-safe objects in application code. Encore then takes care of running your local environments, [provisioning cloud infrastructure](/docs/deploy/infra) and deploying to [your cloud](/docs/deploy/own-cloud) on AWS/GCP.

**See how to use each primitive:**
- [Services and APIs](/docs/primitives/services-and-apis)
- [Databases](/docs/primitives/databases)
- [Cron Jobs](/docs/primitives/cron-jobs)
- [Pub/Sub & Queues](/docs/primitives/pubsub)
- [Caching](/docs/primitives/caching)
- [Secrets](/docs/primitives/secrets)

Check out the [Tutorials](/docs/tutorials) section for examples of complete Encore applications.

<img src="/assets/docs/primitives.png" title="Cloud Primitives" className="noshadow mx-auto d:w-1/2"/>

## Benefits of using Encore for Go

Using the Go SDK to declare infrastructure in application code helps unlock these benefits:

- **Develop new features locally as if the infrastructure is already set up**: Encore automatically compiles your app and sets up the necessary infrastructure on the fly.
- **Faster feedback loops:** With type-safe infrastructure you can identify problems as early as in your code editor, instead of learning about issues via the — much slower — deployment cycle.
- **No manual maintenance required**: There is no need to manually write [IaC](/resources/infrastructure-as-code) configuration, like Terraform, and no need to maintain configuration for multiple environments manually. Encore uses your application code as the source of truth and automatically keeps all environments in sync.
- **One codebase for all environments**: Encore [automatically provisions](/docs/deploy/infra) your local, [preview](/docs/deploy/preview-environments), and cloud environments (using [your own cloud account](/docs/deploy/own-cloud)) on AWS/GCP.
- **Cloud-agnostic by default**: The Infrastructure SDK is an abstraction layer on top of the cloud provider's APIs, so you avoid becoming locked in to a single cloud.
- **Evolve infrastructure without code changes**: As your requirements evolve, you can change and configure the provisioned infrastructure by using Encore's Cloud Dashboard or your cloud provider's console.

## Simplicity without giving up flexibility

While most requirements are met by a common set of infrastructure primitives, sooner or later you will likely need something highly specific to your problem domain. Encore is designed to ensure you can use any cloud infrastructure, even if it's not built into Encore's Infrastructure SDK. This works seamlessly since Encore [provisions infrastructure](/docs/deploy/infra) in your own cloud account, so you can use any of your cloud provider's services as you traditionally would.
